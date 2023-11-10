package controller

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/dbsystel/alertmanager-config-controller/alertmanager"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	alcf "github.com/prometheus/alertmanager/config"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
)

var (
	receiverConst    = "receiver"
	routeConst       = "route"
	inhibitRuleConst = "inhibit rule"
	configConst      = "config"
)

// Controller wrapper for alertmanager
type Controller struct {
	logger log.Logger
	a      alertmanager.APIClient
}

// New creates new Controller instance
func New(a alertmanager.APIClient, logger log.Logger) *Controller {
	controller := &Controller{}
	controller.logger = logger
	controller.a = a
	return controller
}

// Create is called when a configmap is created
func (c *Controller) Create(obj interface{}) {
	configmapObj := obj.(*v1.ConfigMap)
	id := configmapObj.Annotations["alertmanager.net/id"]
	route := configmapObj.Annotations["alertmanager.net/route"]
	receiver := configmapObj.Annotations["alertmanager.net/receiver"]
	inhibitRule := configmapObj.Annotations["alertmanager.net/inhibit_rule"]
	config := configmapObj.Annotations["alertmanager.net/config"]
	key := configmapObj.Annotations["alertmanager.net/key"]
	isAlertmanagerRoute, _ := strconv.ParseBool(route)
	isAlertmanagerReceiver, _ := strconv.ParseBool(receiver)
	isAlertmanagerInhibitRule, _ := strconv.ParseBool(inhibitRule)
	isAlertmanagerConfig, _ := strconv.ParseBool(config)
	alertmanagerID, _ := strconv.Atoi(id)

	if alertmanagerID == c.a.ID &&
		((isAlertmanagerConfig && key == c.a.Key) ||
			isAlertmanagerRoute ||
			isAlertmanagerReceiver ||
			isAlertmanagerInhibitRule) {

		configType := c.findConfigType(isAlertmanagerRoute,
			isAlertmanagerReceiver,
			isAlertmanagerInhibitRule,
			isAlertmanagerConfig)

		c.createConfig(configmapObj, configType)

		c.checkBackupConfigs()

		err := c.buildConfig()
		if err == nil {
			_, err = c.a.Reload()
			if err != nil {
				//nolint:errcheck
				level.Error(c.logger).Log(
					"msg", "Failed to reload alertmanager.yml",
					"err", err.Error(),
					"namespace", configmapObj.Namespace,
					"name", configmapObj.Name,
				)
			} else {
				//nolint:errcheck
				level.Info(c.logger).Log("msg", "Succeeded: Reloaded Alertmanager")
			}
		} else if configType != configConst {
			if configType == routeConst || configType == receiverConst || configType == inhibitRuleConst {
				c.createBackfile(configmapObj, configType)
			}
			c.deleteConfig(configmapObj)
		}
	} else {
		//nolint:errcheck
		level.Debug(c.logger).Log("msg", "Skipping configmap:"+configmapObj.Name)
	}
}

func (c *Controller) findConfigType(isRoute, isReceiver, isInhibitRule, isConfig bool) string {
	check := strconv.FormatBool(isRoute) +
		"-" + strconv.FormatBool(isReceiver) +
		"-" + strconv.FormatBool(isInhibitRule) +
		"-" + strconv.FormatBool(isConfig)
	switch check {
	case "true-false-false-false":
		return routeConst
	case "false-true-false-false":
		return receiverConst
	case "false-false-true-false":
		return inhibitRuleConst
	case "false-false-false-true":
		return "config"
	default:
		return ""
	}
}

func (c *Controller) checkBackupConfigs() {
	files, _ := ioutil.ReadDir(c.a.ConfigPath + "/inhibit-rules")
	if len(files) > 0 {
		//nolint:errcheck
		level.Debug(c.logger).Log("msg", "Checking backup inhibit rules for new receiver...")
		c.checkBackupInhibitRules()
	}
	files, _ = ioutil.ReadDir(c.a.ConfigPath + "/backup-routes")
	if len(files) > 0 {
		//nolint:errcheck
		level.Debug(c.logger).Log("msg", "Checking backup routes for new receiver...")
		c.checkBackupRoutes()
	}
	files, _ = ioutil.ReadDir(c.a.ConfigPath + "/backup-receivers")
	if len(files) > 0 {
		//nolint:errcheck
		level.Debug(c.logger).Log("msg", "Checking backup receivers...")
		c.checkBackupReceivers()
	}
}

// Delete is called when a configmap is deleted
func (c *Controller) Delete(obj interface{}) {
	configmapObj := obj.(*v1.ConfigMap)
	id := configmapObj.Annotations["alertmanager.net/id"]
	route := configmapObj.Annotations["alertmanager.net/route"]
	receiver := configmapObj.Annotations["alertmanager.net/receiver"]
	inhibitRule := configmapObj.Annotations["alertmanager.net/inhibit_rule"]
	isAlertmanagerRoute, _ := strconv.ParseBool(route)
	isAlertmanagerReceiver, _ := strconv.ParseBool(receiver)
	isAlertmanagerInhibitRule, _ := strconv.ParseBool(inhibitRule)
	alertmanagerID, _ := strconv.Atoi(id)

	if alertmanagerID == c.a.ID && (isAlertmanagerReceiver || isAlertmanagerRoute || isAlertmanagerInhibitRule) {
		c.deleteConfig(configmapObj)
		if isAlertmanagerRoute {
			c.deleteBackupFile(configmapObj, routeConst)
		}
		if isAlertmanagerReceiver {
			c.deleteBackupFile(configmapObj, receiverConst)
		}
		if isAlertmanagerInhibitRule {
			c.deleteBackupFile(configmapObj, inhibitRuleConst)
		}

		c.checkBackupConfigs()

		err := c.buildConfig()
		if err == nil {
			_, err = c.a.Reload()
			if err != nil {
				//nolint:errcheck
				level.Error(c.logger).Log(
					"msg", "Failed to reload alertmanager.yml",
					"err", err.Error(),
					"namespace", configmapObj.Namespace,
					"name", configmapObj.Name,
				)
			} else {
				//nolint:errcheck
				level.Info(c.logger).Log("msg", "Succeeded: Reloaded Alertmanager")
			}
		}

	} else {
		//nolint:errcheck
		level.Debug(c.logger).Log("msg", "Skipping configmap:"+configmapObj.Name)
	}

}

// Update is called when a configmap is updated
func (c *Controller) Update(oldobj, newobj interface{}) {
	newConfigmapObj := newobj.(*v1.ConfigMap)
	oldConfigmapObj := oldobj.(*v1.ConfigMap)
	newID := newConfigmapObj.Annotations["alertmanager.net/id"]
	oldID := oldConfigmapObj.Annotations["alertmanager.net/id"]
	route := newConfigmapObj.Annotations["alertmanager.net/route"]
	oldRoute := oldConfigmapObj.Annotations["alertmanager.net/route"]
	receiver := newConfigmapObj.Annotations["alertmanager.net/receiver"]
	oldReceiver := oldConfigmapObj.Annotations["alertmanager.net/receiver"]
	inhibitRule := newConfigmapObj.Annotations["alertmanager.net/inhibit_rule"]
	oldInhibitRule := oldConfigmapObj.Annotations["alertmanager.net/inhibit_rule"]
	config := newConfigmapObj.Annotations["alertmanager.net/config"]
	key := newConfigmapObj.Annotations["alertmanager.net/key"]
	isAlertmanagerRoute, _ := strconv.ParseBool(route)
	isOldAlertmanagerRoute, _ := strconv.ParseBool(oldRoute)
	isAlertmanagerReceiver, _ := strconv.ParseBool(receiver)
	isOldAlertmanagerReceiver, _ := strconv.ParseBool(oldReceiver)
	isAlertmanagerInhibitRule, _ := strconv.ParseBool(inhibitRule)
	isOldAlertmanagerInhibitRule, _ := strconv.ParseBool(oldInhibitRule)
	isAlertmanagerConfig, _ := strconv.ParseBool(config)
	newAlertmanagerID, _ := strconv.Atoi(newID)
	oldAlertmanagerID, _ := strconv.Atoi(oldID)

	if newAlertmanagerID == oldAlertmanagerID && noDifference(oldConfigmapObj, newConfigmapObj) {
		//nolint:errcheck
		level.Debug(c.logger).Log("msg", "Skipping automatically updated configmap:"+newConfigmapObj.Name)
		return
	}
	if (oldAlertmanagerID == c.a.ID || newAlertmanagerID == c.a.ID) &&
		(isOldAlertmanagerRoute ||
			isAlertmanagerRoute ||
			isOldAlertmanagerReceiver ||
			isAlertmanagerReceiver ||
			isOldAlertmanagerInhibitRule ||
			isAlertmanagerConfig ||
			isAlertmanagerInhibitRule) {

		if oldAlertmanagerID == c.a.ID {
			if isOldAlertmanagerReceiver {
				c.deleteConfig(oldConfigmapObj)
				c.deleteBackupFile(oldConfigmapObj, receiverConst)
			}
			if isOldAlertmanagerRoute {
				c.deleteConfig(oldConfigmapObj)
				c.deleteBackupFile(oldConfigmapObj, routeConst)
			}
			if isOldAlertmanagerInhibitRule {
				c.deleteConfig(oldConfigmapObj)
				c.deleteBackupFile(oldConfigmapObj, inhibitRuleConst)
			}
		}

		newConfigType := c.findConfigType(isAlertmanagerRoute,
			isAlertmanagerReceiver,
			isAlertmanagerInhibitRule,
			isAlertmanagerConfig)

		if newAlertmanagerID == c.a.ID {
			if (isAlertmanagerReceiver ||
				isAlertmanagerRoute ||
				isAlertmanagerInhibitRule) ||
				(isAlertmanagerConfig && key == c.a.Key) {

				c.createConfig(newConfigmapObj, newConfigType)
			}
		}

		c.checkBackupConfigs()

		err := c.buildConfig()
		if err == nil {
			_, err = c.a.Reload()
			if err != nil {
				//nolint:errcheck
				level.Error(c.logger).Log(
					"msg", "Failed to reload alertmanager.yml",
					"err", err.Error(),
					"namespace", newConfigmapObj.Namespace,
					"name", newConfigmapObj.Name,
				)
			} else {
				//nolint:errcheck
				level.Info(c.logger).Log("msg", "Succeeded: Reloaded Alertmanager")
			}
		} else if newAlertmanagerID == c.a.ID {
			if newConfigType != configConst {
				if newConfigType == routeConst || newConfigType == receiverConst || newConfigType == inhibitRuleConst {
					c.createBackfile(newConfigmapObj, newConfigType)
				}
				c.deleteConfig(newConfigmapObj)
			}
		}
	} else {
		//nolint:errcheck
		level.Debug(c.logger).Log("msg", "Skipping configmap:"+newConfigmapObj.Name)
	}
}

// save configs(receivers, routes, inhibitrules, config template) into storage
func (c *Controller) createConfig(configmapObj *v1.ConfigMap, configType string) {
	var err error
	path := ""

	switch configType {
	case routeConst:
		path = c.a.ConfigPath + "/routes/"
	case receiverConst:
		path = c.a.ConfigPath + "/receivers/"
	case inhibitRuleConst:
		path = c.a.ConfigPath + "/inhibit-rules/"
	case configConst:
		path = filepath.Dir(c.a.ConfigTemplate) + "/"
	default:
		return
	}

	if _, err = os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, 0766)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log("msg", "Failed to create directory", "err", err.Error())
		}
	}

	for k, v := range configmapObj.Data {
		filename := ""
		if configType == configConst {
			filename = k
		} else {
			filename = configmapObj.Namespace + "-" + configmapObj.Name + "-" + k
		}

		if configType == routeConst {
			v = c.addContinueIfNotExist(v)
		}

		//nolint:errcheck
		level.Info(c.logger).Log(
			"msg", "Creating "+configType+": "+k,
			"namespace", configmapObj.Namespace,
			"name", configmapObj.Name,
		)
		err = ioutil.WriteFile(path+filename, []byte(v), 0644)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log(
				"msg", "Failed to create "+configType+": "+k,
				"namespace", configmapObj.Namespace,
				"name", configmapObj.Name,
			)
		}
	}
}

func (c *Controller) addContinueIfNotExist(routeString string) string {
	m := make([]map[string]interface{}, 1)

	err := yaml.Unmarshal([]byte(routeString), &m)
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Format error in route string: "+routeString, "err", err.Error())
		return routeString
	}

	for _, route := range m {
		if len(route) == 0 {
			level.Warn(c.logger).Log("msg", "One of your route config is empty")
			continue
		}
		route["continue"] = true
	}

	v, err := yaml.Marshal(&m)
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Format error in route yaml", "err", err.Error())
	}

	return string(v)
}

// backup currently unavailable configs for further usage
func (c *Controller) createBackfile(configmapObj *v1.ConfigMap, configType string) {
	path := c.a.ConfigPath + "/backup-" + configType + "s/"
	var err error
	if _, err = os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, 0766)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log("msg", "Failed to create backup directory", "err", err.Error())
		}
	}
	for k, v := range configmapObj.Data {
		filename := configmapObj.Namespace + "-" + configmapObj.Name + "-" + k
		if configType == routeConst {
			v = c.addContinueIfNotExist(v)
		}
		//nolint:errcheck
		level.Debug(c.logger).Log(
			"msg", "Backup "+configType+": "+k,
			"namespace", configmapObj.Namespace,
			"name", configmapObj.Name,
		)
		err = ioutil.WriteFile(path+filename, []byte(v), 0644)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log("msg", "Failed to backup "+configType+": "+k, "err", err.Error())
		}
	}
}

// go through backup routes to check if any of them can be used now
func (c *Controller) checkBackupRoutes() {
	routeFiles, err := filepath.Glob(c.a.ConfigPath + "/backup-routes/*")
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to read backup routes", "err", err.Error())
	}

	routes := ""
	receivers := c.readConfigs("receivers")
	inhibitRules := c.readConfigs("inhibit-rules")

	var alertmanagerConfig alertmanager.Config
	alertmanagerConfig.Receivers = receivers
	alertmanagerConfig.InhibitRules = inhibitRules

	configTemplate, err := ioutil.ReadFile(c.a.ConfigTemplate)
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to read template: "+c.a.ConfigTemplate, "err", err.Error())
	}

	t, err := template.New("alertmanager.yml").Parse(string(configTemplate))
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to parse template", "err", err.Error())
	}

	for _, routeFile := range routeFiles {
		route, err := ioutil.ReadFile(routeFile)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log("msg", "Failed to read route: "+routeFile, "err", err.Error())
		}
		routes = string(route)

		alertmanagerConfig.Routes = strings.Replace(routes, "\n", "\n  ", -1)

		var tpl bytes.Buffer
		err = t.Execute(&tpl, alertmanagerConfig)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log("msg", "Failed to template alertmanager config", "err", err.Error())
		}
		_, configErr := alcf.Load(tpl.String())
		if configErr == nil {
			c.copyFile(routeFile, c.a.ConfigPath+"/routes/"+filepath.Base(routeFile))
			err = os.Remove(routeFile)
			if err != nil {
				//nolint:errcheck
				level.Error(c.logger).Log("msg", "Failed to delete route: "+routeFile, "err", err.Error())
			}
			//nolint:errcheck
			level.Debug(c.logger).Log("msg", "Route is available", "route", routeFile)
			c.checkBackupConfigs()
			break
		} else {
			//nolint:errcheck
			level.Debug(c.logger).Log("msg", "Route is unavailable", "route", routeFile, "err", configErr.Error())
		}
	}
}

// go through backup receivers to check if any of them can be used now
func (c *Controller) checkBackupReceivers() {
	receiverPath, err := filepath.Glob(c.a.ConfigPath + "/backup-receivers/*")
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to read backup receivers", "err", err.Error())
	}

	routes := c.readConfigs("routes")
	receivers := c.readConfigs("receivers")
	inhibitRules := c.readConfigs("inhibit-rules")

	var alertmanagerConfig alertmanager.Config
	alertmanagerConfig.Routes = strings.Replace(routes, "\n", "\n  ", -1)
	alertmanagerConfig.InhibitRules = inhibitRules

	configTemplate, err := ioutil.ReadFile(c.a.ConfigTemplate)
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to read template: "+c.a.ConfigTemplate, "err", err.Error())
	}

	t, err := template.New("alertmanager.yml").Parse(string(configTemplate))
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to parse template", "err", err.Error())
	}

	for _, receiverFile := range receiverPath {
		receiver, err := ioutil.ReadFile(receiverFile)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log("msg", "Failed to read receiver: "+receiverFile, "err", err.Error())
		}
		newReceivers := receivers + string(receiver)

		alertmanagerConfig.Receivers = newReceivers
		var tpl bytes.Buffer
		err = t.Execute(&tpl, alertmanagerConfig)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log("msg", "Failed to template alertmanager config", "err", err.Error())
		}
		_, configErr := alcf.Load(tpl.String())
		if configErr == nil {
			c.copyFile(receiverFile, c.a.ConfigPath+"/receivers/"+filepath.Base(receiverFile))
			err = os.Remove(receiverFile)
			if err != nil {
				//nolint:errcheck
				level.Error(c.logger).Log("msg", "Failed to delete receiver: "+receiverFile, "err", err.Error())
			}
			//nolint:errcheck
			level.Debug(c.logger).Log("msg", "Receiver is available", "receiver", receiverFile)
			c.checkBackupConfigs()
			break
		} else {
			//nolint:errcheck
			level.Debug(c.logger).Log("msg", "Route is unavailable", "receiver", receiverFile, "err", configErr.Error())
		}
	}
}

// format config file from routs, receivers, inhibit rules and config template
func (c *Controller) buildConfig() error {
	configTemplate, err := ioutil.ReadFile(c.a.ConfigTemplate)
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to read template: "+c.a.ConfigTemplate, "err", err.Error())
	}

	routes := c.readConfigs("routes")
	receivers := c.readConfigs("receivers")
	inhibitRules := c.readConfigs("inhibit-rules")
	var alertmanagerConfig alertmanager.Config

	alertmanagerConfig.Routes = strings.Replace(routes, "\n", "\n  ", -1)
	alertmanagerConfig.Receivers = receivers
	alertmanagerConfig.InhibitRules = inhibitRules

	t, err := template.New("alertmanager.yml").Parse(string(configTemplate))
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to parse template", "err", err.Error())
	}

	var tpl bytes.Buffer
	err = t.Execute(&tpl, alertmanagerConfig)
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to template alertmanager config", "err", err.Error())
	}
	_, configErr := alcf.Load(tpl.String())
	if configErr == nil {
		f, err := os.Create(c.a.ConfigPath + "/alertmanager.yml")
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log("msg", "Failed to create alertmanager.yml", "err", err.Error())
		}
		defer f.Close()
		err = t.Execute(f, alertmanagerConfig)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log("msg", "Failed to template alertmanager config", "err", err.Error())
		}
	} else {
		//nolint:errcheck
		level.Error(c.logger).Log("err", configErr.Error())
		c.a.ErrorCount.Inc()
	}
	return configErr
}

// read config files from storage
func (c *Controller) readConfigs(style string) string {
	configFiles, err := filepath.Glob(c.a.ConfigPath + "/" + style + "/*")
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to read "+style, "err", err.Error())
	}

	configs := ""
	for _, configFile := range configFiles {
		config, err := ioutil.ReadFile(configFile)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log("msg", "Failed to read "+style+" file "+configFile, "err", err.Error())
		}
		configs = configs + string(config) + "\n"
	}

	return configs
}

// remove config files from storage
func (c *Controller) deleteConfig(configmapObj *v1.ConfigMap) {
	var err error
	route := configmapObj.Annotations["alertmanager.net/route"]
	receiver := configmapObj.Annotations["alertmanager.net/receiver"]
	inhibitRule := configmapObj.Annotations["alertmanager.net/inhibit_rule"]
	isAlertmanagerRoute, _ := strconv.ParseBool(route)
	isAlertmanagerReceiver, _ := strconv.ParseBool(receiver)
	isAlertmanagerInhibitRule, _ := strconv.ParseBool(inhibitRule)
	path := ""
	configType := c.findConfigType(isAlertmanagerRoute, isAlertmanagerReceiver, isAlertmanagerInhibitRule, false)
	path = c.a.ConfigPath + "/" + configType + "s/"

	for k := range configmapObj.Data {
		filename := configmapObj.Namespace + "-" + configmapObj.Name + "-" + k
		//nolint:errcheck
		level.Info(c.logger).Log(
			"msg", "Deleting "+configType+": "+k,
			"namespace", configmapObj.Namespace,
			"name", configmapObj.Name,
		)
		err = os.Remove(path + filename)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log(
				"msg", "Failed to delete "+configType+": "+k,
				"namespace", configmapObj.Namespace,
				"name", configmapObj.Name,
				"err", err.Error(),
			)
		}
	}
}

// remove config files from backup storage
func (c *Controller) deleteBackupFile(configmapObj *v1.ConfigMap, configType string) {
	for k := range configmapObj.Data {
		filename := configmapObj.Namespace + "-" + configmapObj.Name + "-" + k
		//nolint:errcheck
		level.Debug(c.logger).Log("mag", "Delete backup "+configType+" if it is existed")
		if _, err := os.Stat(c.a.ConfigPath + "/backup-" + configType + "s/" + filename); !os.IsNotExist(err) {
			err := os.Remove(c.a.ConfigPath + "/backup-" + configType + "s/" + filename)
			if err != nil {
				//nolint:errcheck
				level.Error(c.logger).Log("msg", "Failed to delete backup "+configType+": "+filename, "err", err.Error())
			}
		} else {
			//nolint:errcheck
			level.Debug(c.logger).Log("msg", "Backup "+configType+" does not exist")
		}
	}
}

// are two configmaps same
func noDifference(newConfigMap *v1.ConfigMap, oldConfigMap *v1.ConfigMap) bool {
	if len(newConfigMap.Data) != len(oldConfigMap.Data) {
		return false
	}
	for k, v := range newConfigMap.Data {
		if v != oldConfigMap.Data[k] {
			return false
		}
	}
	if len(newConfigMap.Annotations) != len(oldConfigMap.Annotations) {
		return false
	}
	for k, v := range newConfigMap.Annotations {
		if v != oldConfigMap.Annotations[k] {
			return false
		}
	}
	return true
}

// copy file from sourceFile to targetFile
func (c *Controller) copyFile(sourceFile string, targetFile string) {
	source, err := os.Open(sourceFile)
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to read file", "file", sourceFile, "err", err.Error())
	}
	defer source.Close()

	dest, err := os.Create(targetFile)
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to create file", "file", sourceFile, "err", err.Error())
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to copy file", "file", sourceFile, "err", err.Error())
	}
}

func (c *Controller) checkBackupInhibitRules() {
	inhibitRulePath, err := filepath.Glob(c.a.ConfigPath + "/backup-inhibit-rules/*")
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to read backup inhibit rules", "err", err.Error())
	}

	routes := c.readConfigs("routes")
	receivers := c.readConfigs("receivers")
	inhibitRules := c.readConfigs("inhibit-rules")

	var alertmanagerConfig alertmanager.Config
	alertmanagerConfig.Routes = strings.Replace(routes, "\n", "\n  ", -1)
	alertmanagerConfig.Receivers = receivers

	configTemplate, err := ioutil.ReadFile(c.a.ConfigTemplate)
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to read template: "+c.a.ConfigTemplate, "err", err.Error())
	}

	t, err := template.New("alertmanager.yml").Parse(string(configTemplate))
	if err != nil {
		//nolint:errcheck
		level.Error(c.logger).Log("msg", "Failed to parse template", "err", err.Error())
	}

	for _, inhibitRuleFile := range inhibitRulePath {
		inhibitRule, err := ioutil.ReadFile(inhibitRuleFile)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log("msg", "Failed to read inhibit rule: "+inhibitRuleFile, "err", err.Error())
		}
		newInhibitRules := inhibitRules + string(inhibitRule)

		alertmanagerConfig.InhibitRules = newInhibitRules
		var tpl bytes.Buffer
		err = t.Execute(&tpl, alertmanagerConfig)
		if err != nil {
			//nolint:errcheck
			level.Error(c.logger).Log("msg", "Failed to template alertmanager config", "err", err.Error())
		}
		_, configErr := alcf.Load(tpl.String())
		if configErr == nil {
			c.copyFile(inhibitRuleFile, c.a.ConfigPath+"/inhibit-rules/"+filepath.Base(inhibitRuleFile))
			err = os.Remove(inhibitRuleFile)
			if err != nil {
				//nolint:errcheck
				level.Error(c.logger).Log("msg", "Failed to delete inhibitRule: "+inhibitRuleFile, "err", err.Error())
			}
			//nolint:errcheck
			level.Debug(c.logger).Log("msg", "Inhibit rule is available", "inhibitRule", inhibitRuleFile)
		} else {
			//nolint:errcheck
			level.Debug(c.logger).Log("msg", "Inhibit rule is unavailable", "inhibitRule", inhibitRuleFile, "err", configErr)
		}
	}

}
