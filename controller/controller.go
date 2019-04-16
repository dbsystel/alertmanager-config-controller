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
	"k8s.io/api/core/v1"
)

type Controller struct {
	logger log.Logger
	a      alertmanager.APIClient
}

// create new Controller instance
func New(a alertmanager.APIClient, logger log.Logger) *Controller {
	controller := &Controller{}
	controller.logger = logger
	controller.a = a
	return controller
}

// do something when a configmap created
func (c *Controller) Create(obj interface{}) {
	configmapObj   := obj.(*v1.ConfigMap)
	id, _          := configmapObj.Annotations["alertmanager.net/id"]
	route, _       := configmapObj.Annotations["alertmanager.net/route"]
	receiver, _    := configmapObj.Annotations["alertmanager.net/receiver"]
	inhibitRule, _ := configmapObj.Annotations["alertmanager.net/inhibit_rule"]
	config, _      := configmapObj.Annotations["alertmanager.net/config"]
	key, _         := configmapObj.Annotations["alertmanager.net/key"]
	isAlertmanagerRoute, _       := strconv.ParseBool(route)
	isAlertmanagerReceiver, _    := strconv.ParseBool(receiver)
	isAlertmanagerInhibitRule, _ := strconv.ParseBool(inhibitRule)
	isAlertmanagerConfig, _      := strconv.ParseBool(config)
	alertmanagerId, _            := strconv.Atoi(id)

	if alertmanagerId == c.a.Id && ((isAlertmanagerConfig && key == c.a.Key) || isAlertmanagerRoute || isAlertmanagerReceiver || isAlertmanagerInhibitRule) {
		c.createConfig(configmapObj)

		c.checkBackupConfigs()

		err := c.buildConfig()
		if err == nil {
			err, _ = c.a.Reload()
			if err != nil {
				level.Error(c.logger).Log(
					"msg", "Failed to reload alertmanager.yml",
					"err", err.Error(),
					"namespace", configmapObj.Namespace,
					"name", configmapObj.Name,
				)
			} else {
				level.Info(c.logger).Log("msg", "Succeeded: Reloaded Alertmanager")
			}
		} else if isAlertmanagerRoute {
			c.createBackfile(configmapObj, "route")
			c.deleteConfig(configmapObj)
		} else if isAlertmanagerReceiver {
			c.createBackfile(configmapObj, "receiver")
			c.deleteConfig(configmapObj)
		} else if isAlertmanagerInhibitRule {
			c.createBackfile(configmapObj, "inhibit rule")
			c.deleteConfig(configmapObj)
		} else if !isAlertmanagerConfig {
			c.deleteConfig(configmapObj)
		}
	} else {
		level.Debug(c.logger).Log("msg", "Skipping configmap:"+configmapObj.Name)
	}
}

func (c *Controller) checkBackupConfigs() {
	files, _ := ioutil.ReadDir(c.a.ConfigPath + "/inhibit-rules")
	if len(files) > 0 {
		level.Debug(c.logger).Log("msg", "Checking backup inhibit rules for new receiver...")
		c.checkBackupInhibitRules()
	}
	files, _ = ioutil.ReadDir(c.a.ConfigPath + "/backup-routes")
	if len(files) > 0 {
		level.Debug(c.logger).Log("msg", "Checking backup routes for new receiver...")
		c.checkBackupRoutes()
	}
	files, _ = ioutil.ReadDir(c.a.ConfigPath + "/backup-receivers")
	if len(files) > 0 {
		level.Debug(c.logger).Log("msg", "Checking backup receivers...")
		c.checkBackupReceivers()
	}
}

// do something when a configmap deleted
func (c *Controller) Delete(obj interface{}) {
	configmapObj := obj.(*v1.ConfigMap)
	id, _          := configmapObj.Annotations["alertmanager.net/id"]
	route, _       := configmapObj.Annotations["alertmanager.net/route"]
	receiver, _    := configmapObj.Annotations["alertmanager.net/receiver"]
	inhibitRule, _ := configmapObj.Annotations["alertmanager.net/inhibit_rule"]
	isAlertmanagerRoute, _       := strconv.ParseBool(route)
	isAlertmanagerReceiver, _    := strconv.ParseBool(receiver)
	isAlertmanagerInhibitRule, _ := strconv.ParseBool(inhibitRule)
	alertmanagerId,_ := strconv.Atoi(id)

	if alertmanagerId == c.a.Id && (isAlertmanagerReceiver || isAlertmanagerRoute || isAlertmanagerInhibitRule) {
		c.deleteConfig(configmapObj)
		if isAlertmanagerRoute {
			c.deleteBackupFile(configmapObj, "route")
		}
		if isAlertmanagerReceiver {
			c.deleteBackupFile(configmapObj, "receiver")
		}
		if isAlertmanagerInhibitRule {
			c.deleteBackupFile(configmapObj, "inhibit rule")
		}

		c.checkBackupConfigs()

		err := c.buildConfig()
		if err == nil {
			err, _ = c.a.Reload()
			if err != nil {
				level.Error(c.logger).Log(
					"msg", "Failed to reload alertmanager.yml",
					"err", err.Error(),
					"namespace", configmapObj.Namespace,
					"name", configmapObj.Name,
				)			} else {
				level.Info(c.logger).Log("msg", "Succeeded: Reloaded Alertmanager")
			}
		}

	} else {
		level.Debug(c.logger).Log("msg", "Skipping configmap:" + configmapObj.Name)
	}

}

// do something when a configmap updated
func (c *Controller) Update(oldobj, newobj interface{}) {
	newConfigmapObj := newobj.(*v1.ConfigMap)
	oldConfigmapObj := oldobj.(*v1.ConfigMap)
	newId, _       := newConfigmapObj.Annotations["alertmanager.net/id"]
	oldId, _       := oldConfigmapObj.Annotations["alertmanager.net/id"]
	route, _       := newConfigmapObj.Annotations["alertmanager.net/route"]
	receiver, _    := newConfigmapObj.Annotations["alertmanager.net/receiver"]
	inhibitRule, _ := newConfigmapObj.Annotations["alertmanager.net/inhibit_rule"]
	config, _ := newConfigmapObj.Annotations["alertmanager.net/config"]
	key, _    := newConfigmapObj.Annotations["alertmanager.net/key"]
	isAlertmanagerRoute, _       := strconv.ParseBool(route)
	isAlertmanagerReceiver, _    := strconv.ParseBool(receiver)
	isAlertmanagerInhibitRule, _ := strconv.ParseBool(inhibitRule)
	isAlertmanagerConfig, _      := strconv.ParseBool(config)
	newAlertmanagerId, _ := strconv.Atoi(newId)
	oldAlertmanagerId, _ := strconv.Atoi(oldId)

	if newAlertmanagerId == oldAlertmanagerId && noDifference(oldConfigmapObj, newConfigmapObj) {
		level.Debug(c.logger).Log("msg", "Skipping automatically updated configmap:" + newConfigmapObj.Name)
		return
	}
	if (oldAlertmanagerId == c.a.Id || newAlertmanagerId == c.a.Id) && (isAlertmanagerRoute || isAlertmanagerReceiver || isAlertmanagerConfig || isAlertmanagerInhibitRule){
		if isAlertmanagerReceiver {
			if oldAlertmanagerId == c.a.Id {
				c.deleteConfig(oldConfigmapObj)
				c.deleteBackupFile(oldConfigmapObj, "receiver")
			}
			if newAlertmanagerId == c.a.Id {
				c.createConfig(newConfigmapObj)
			}
		} else if isAlertmanagerRoute {
			if oldAlertmanagerId == c.a.Id {
				c.deleteConfig(oldConfigmapObj)
				c.deleteBackupFile(oldConfigmapObj, "route")
			}
			if newAlertmanagerId == c.a.Id {
				c.createConfig(newConfigmapObj)
			}
		} else if isAlertmanagerInhibitRule {
			if oldAlertmanagerId == c.a.Id {
				c.deleteConfig(oldConfigmapObj)
				c.deleteBackupFile(oldConfigmapObj, "inhibit rule")
			}
			if newAlertmanagerId == c.a.Id {
				c.createConfig(newConfigmapObj)
			}
		} else if isAlertmanagerConfig && key == c.a.Key {
			c.createConfig(newConfigmapObj)
		}

		c.checkBackupConfigs()

		err := c.buildConfig()
		if err == nil {
			err, _ = c.a.Reload()
			if err != nil {
				level.Error(c.logger).Log(
					"msg", "Failed to reload alertmanager.yml",
					"err", err.Error(),
					"namespace", newConfigmapObj.Namespace,
					"name", newConfigmapObj.Name,
				)			} else {
				level.Info(c.logger).Log("msg", "Succeeded: Reloaded Alertmanager")
			}
		} else if isAlertmanagerRoute {
			c.createBackfile(newConfigmapObj, "route")
			c.deleteConfig(newConfigmapObj)
		} else if isAlertmanagerReceiver {
			c.createBackfile(newConfigmapObj, "receiver")
			c.deleteConfig(newConfigmapObj)
		} else if isAlertmanagerInhibitRule {
			c.createBackfile(newConfigmapObj, "inhibit rule")
			c.deleteConfig(newConfigmapObj)
		} else if !isAlertmanagerConfig{
			c.deleteConfig(newConfigmapObj)
		}
	} else {
		level.Debug(c.logger).Log("msg", "Skipping configmap:" + newConfigmapObj.Name)
	}
}

// save configs(receivers, routes, inhibitrules, config template) into storage
func (c *Controller) createConfig(configmapObj *v1.ConfigMap) {
	var err error
	route, _       := configmapObj.Annotations["alertmanager.net/route"]
	receiver, _    := configmapObj.Annotations["alertmanager.net/receiver"]
	inhibitRule, _ := configmapObj.Annotations["alertmanager.net/inhibit_rule"]
	config, _      := configmapObj.Annotations["alertmanager.net/config"]
	isAlertmanagerRoute, _       := strconv.ParseBool(route)
	isAlertmanagerReceiver, _    := strconv.ParseBool(receiver)
	isAlertmanagerInhibitRule, _ := strconv.ParseBool(inhibitRule)
	isAlertmanagerConfig, _      := strconv.ParseBool(config)
	path := ""
	typ  := ""
	if isAlertmanagerConfig {
		path = filepath.Dir(c.a.ConfigTemplate) + "/"
		typ  = "config"
	} else if isAlertmanagerReceiver {
		path = c.a.ConfigPath + "/receivers/"
		typ  = "receiver"
	} else if isAlertmanagerRoute {
		path = c.a.ConfigPath + "/routes/"
		typ  = "route"
	} else if isAlertmanagerInhibitRule {
		path = c.a.ConfigPath + "/inhibit-rules/"
		typ  = "inhibit rule"
	}

	if _, err = os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, 0766)
		if err != nil {
			level.Error(c.logger).Log("msg", "Failed to create directory", "err", err)
		}
	}



	for k, v := range configmapObj.Data {
		filename := ""
		if typ == "config" {
			filename = k
		} else {
			filename = configmapObj.Namespace + "-" + configmapObj.Name + "-" + k
		}

		level.Info(c.logger).Log(
			"msg", "Creating " + typ + ": " + k,
			"namespace", configmapObj.Namespace,
			"name", configmapObj.Name,
		)
		err = ioutil.WriteFile(path + filename, []byte(v), 0644)
		if err != nil {
			level.Error(c.logger).Log(
				"msg", "Failed to create " + typ + ": " + k,
				"namespace", configmapObj.Namespace,
				"name", configmapObj.Name,
			)
		}
	}
}

// backup currently unavailable configs for further usage
func (c *Controller) createBackfile(configmapObj *v1.ConfigMap, typ string) {
	var err error
	if typ == "route" {
		path := c.a.ConfigPath + "/backup-routes/"
		if _, err = os.Stat(path); os.IsNotExist(err) {
			err = os.MkdirAll(path, 0766)
			if err != nil {
				level.Error(c.logger).Log("msg", "Failed to create backup directory", "err", err)
			}
		}
		for k, v := range configmapObj.Data {
			filename := configmapObj.Namespace + "-" + configmapObj.Name + "-" + k
			level.Debug(c.logger).Log(
				"msg", "Backup route: " + k + ", and waiting for receiver",
				"namespace", configmapObj.Namespace,
				"name", configmapObj.Name,
			)
			err = ioutil.WriteFile(path + filename, []byte(v), 0644)
			if err != nil {
				level.Error(c.logger).Log("msg", "Failed to backup route: " + k, "err", err.Error())
			}
		}
	} else if typ == "receiver" {
		path := c.a.ConfigPath + "/backup-receivers/"
		if _, err = os.Stat(path); os.IsNotExist(err) {
			err = os.MkdirAll(path, 0766)
			if err != nil {
				level.Error(c.logger).Log("msg", "Failed to create backup directory", "err", err)
			}
		}
		for k, v := range configmapObj.Data {
			filename := configmapObj.Namespace + "-" + configmapObj.Name + "-" + k
			level.Debug(c.logger).Log(
				"msg", "Backup receiver: " + k,
				"namespace", configmapObj.Namespace,
				"name", configmapObj.Name,
			)
			err = ioutil.WriteFile(path + filename, []byte(v), 0644)
			if err != nil {
				level.Error(c.logger).Log("msg", "Failed to backup receiver: " + k, "err", err.Error())
			}
		}
	} else if typ == "inhibit rule" {
		path := c.a.ConfigPath + "/backup-inhibit-rules/"
		if _, err = os.Stat(path); os.IsNotExist(err) {
			err = os.MkdirAll(path, 0766)
			if err != nil {
				level.Error(c.logger).Log("msg", "Failed to create backup directory", "err", err)
			}
		}
		for k, v := range configmapObj.Data {
			filename := configmapObj.Namespace + "-" + configmapObj.Name + "-" + k
			level.Debug(c.logger).Log(
				"msg", "Backup inhibit-rule: " + k,
				"namespace", configmapObj.Namespace,
				"name", configmapObj.Name,
			)
			err = ioutil.WriteFile(path + filename, []byte(v), 0644)
			if err != nil {
				level.Error(c.logger).Log("msg", "Failed to backup inhibit-rule: " + k, "err", err.Error())
			}
		}
	}

}

// go through backup routes to check if any of them can be used now
func (c *Controller) checkBackupRoutes() {
	routeFiles, err := filepath.Glob(c.a.ConfigPath + "/backup-routes/*")
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to read backup routes", "err", err.Error())
	}

	routes       := ""
	receivers    := c.readConfigs("receivers")
    inhibitRules := c.readConfigs("inhibit-rules")

	var alertmanagerConfig alertmanager.AlertmanagerConfig
	alertmanagerConfig.Receivers    = receivers
    alertmanagerConfig.InhibitRules = inhibitRules

	configTemplate, err := ioutil.ReadFile(c.a.ConfigTemplate)
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to read template: " + c.a.ConfigTemplate, "err", err.Error())
	}

	t, err := template.New("alertmanager.yml").Parse(string(configTemplate))
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to parse template" , "err", err.Error())
	}

	for _, routeFile := range routeFiles {
		route, err := ioutil.ReadFile(routeFile)
		if err != nil {
			level.Error(c.logger).Log("msg", "Failed to read route: " + routeFile, "err", err.Error())
		}
		routes = string(route)

		alertmanagerConfig.Routes    = strings.Replace(routes,"\n", "\n  ", -1)

		var tpl bytes.Buffer
		err = t.Execute(&tpl, alertmanagerConfig)
		_, configErr := alcf.Load(tpl.String())
		if configErr == nil{
			c.copyFile(routeFile, c.a.ConfigPath + "/routes/" + filepath.Base(routeFile))
			err = os.Remove(routeFile)
			if err != nil {
				level.Error(c.logger).Log("msg", "Failed to delete route: " + routeFile, "err", err.Error())
			}
			level.Debug(c.logger).Log("msg", "Route is available", "route", routeFile)
		} else {
			level.Debug(c.logger).Log("msg", "Route is unavailable", "route", routeFile, "err", configErr)
		}
	}
}

// go through backup receivers to check if any of them can be used now
func (c *Controller) checkBackupReceivers() {
	receiverPath, err := filepath.Glob(c.a.ConfigPath + "/backup-receivers/*")
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to read backup receivers", "err", err.Error())
	}

	routes       := c.readConfigs("routes")
	receivers    := c.readConfigs("receivers")
    inhibitRules := c.readConfigs("inhibit-rules")

	var alertmanagerConfig alertmanager.AlertmanagerConfig
	alertmanagerConfig.Routes = strings.Replace(routes,"\n", "\n  ", -1)
    alertmanagerConfig.InhibitRules = inhibitRules
    
	configTemplate, err := ioutil.ReadFile(c.a.ConfigTemplate)
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to read template: " + c.a.ConfigTemplate, "err", err.Error())
	}

	t, err := template.New("alertmanager.yml").Parse(string(configTemplate))
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to parse template" , "err", err.Error())
	}

	for _, receiverFile := range receiverPath {
		receiver, err := ioutil.ReadFile(receiverFile)
		if err != nil {
			level.Error(c.logger).Log("msg", "Failed to read receiver: " + receiverFile, "err", err.Error())
		}
		newReceivers := receivers + string(receiver)

		alertmanagerConfig.Receivers = newReceivers
		var tpl bytes.Buffer
		err = t.Execute(&tpl, alertmanagerConfig)
		_, configErr := alcf.Load(tpl.String())
		if configErr == nil{
			c.copyFile(receiverFile, c.a.ConfigPath + "/receivers/" + filepath.Base(receiverFile))
			err = os.Remove(receiverFile)
			if err != nil {
				level.Error(c.logger).Log("msg", "Failed to delete receiver: " + receiverFile, "err", err.Error())
			}
			level.Debug(c.logger).Log("msg", "Receiver is available", "receiver", receiverFile)
		} else {
			level.Debug(c.logger).Log("msg", "Route is unavailable", "receiver", receiverFile, "err", configErr)
		}
	}
}

// format config file from routs, receivers, inhibit rules and config template
func (c *Controller) buildConfig() error {
	configTemplate, err := ioutil.ReadFile(c.a.ConfigTemplate)
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to read template: " + c.a.ConfigTemplate, "err", err.Error())
	}

	routes       := c.readConfigs("routes")
	receivers    := c.readConfigs("receivers")
    inhibitRules := c.readConfigs("inhibit-rules")
	var alertmanagerConfig alertmanager.AlertmanagerConfig

	alertmanagerConfig.Routes    = strings.Replace(routes,"\n", "\n  ", -1)
	alertmanagerConfig.Receivers = receivers
    alertmanagerConfig.InhibitRules = inhibitRules

	t, err := template.New("alertmanager.yml").Parse(string(configTemplate))
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to parse template" , "err", err.Error())
	}

	var tpl bytes.Buffer
	err = t.Execute(&tpl, alertmanagerConfig)
	_, configErr := alcf.Load(tpl.String())
	if configErr == nil{
		f, err := os.Create(c.a.ConfigPath + "/alertmanager.yml")
		if err != nil {
			level.Error(c.logger).Log("msg", "Failed to create alertmanager.yml" , "err", err.Error())
		}
		defer f.Close()
		err = t.Execute(f, alertmanagerConfig)
	} else {
		level.Error(c.logger).Log("err", configErr.Error())
	}
	return configErr
}

// read config files from storage
func (c *Controller) readConfigs(style string) string {
	configFiles, err := filepath.Glob(c.a.ConfigPath + "/" + style + "/*")
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to read " + style, "err", err.Error())
	}

	configs := ""
	for _, configFile := range configFiles {
		config, err := ioutil.ReadFile(configFile)
		if err != nil {
			level.Error(c.logger).Log("msg", "Failed to read " + style + " file " + configFile , "err", err.Error())
		}
		configs = configs + string(config) + "\n"
	}

	return configs
}

// remove config files from storage
func (c *Controller) deleteConfig(configmapObj *v1.ConfigMap) {
	var err error
	route, _       := configmapObj.Annotations["alertmanager.net/route"]
	receiver, _    := configmapObj.Annotations["alertmanager.net/receiver"]
	inhibitRule, _ := configmapObj.Annotations["alertmanager.net/inhibit_rule"]
	isAlertmanagerRoute, _       := strconv.ParseBool(route)
	isAlertmanagerReceiver, _    := strconv.ParseBool(receiver)
	isAlertmanagerInhibitRule, _ := strconv.ParseBool(inhibitRule)
	path := ""
	typ  := ""
	if isAlertmanagerReceiver {
		path = c.a.ConfigPath + "/receivers/"
		typ  = "receiver"
	} else if isAlertmanagerRoute {
		path = c.a.ConfigPath + "/routes/"
		typ  = "route"
	} else if isAlertmanagerInhibitRule {
		path = c.a.ConfigPath + "/inhibit-rules/"
		typ  = "inhibit rule"
	}

	for k := range configmapObj.Data {
		filename := configmapObj.Namespace + "-" + configmapObj.Name + "-" + k
		level.Info(c.logger).Log(
			"msg", "Deleting " + typ + ": " + k,
			"namespace", configmapObj.Namespace,
			"name", configmapObj.Name,
		)
		err = os.Remove(path + filename)
		if err != nil {
			level.Error(c.logger).Log(
				"msg", "Failed to delete " + typ + ": " + k,
				"namespace", configmapObj.Namespace,
				"name", configmapObj.Name,
				"err", err.Error(),
			)
		}
	}
}

// remove config files from backup storage
func (c *Controller) deleteBackupFile(configmapObj *v1.ConfigMap, typ string) {
	if typ == "receiver" {
		for k := range configmapObj.Data {
			filename := configmapObj.Namespace + "-" + configmapObj.Name + "-" + k
			level.Debug(c.logger).Log("mag", "Delete backup receiver if it is existed")
			if _, err := os.Stat(c.a.ConfigPath + "/backup-receivers/" + filename); !os.IsNotExist(err) {
				err := os.Remove(c.a.ConfigPath + "/backup-receivers/" + filename)
				if err != nil {
					level.Error(c.logger).Log("msg", "Failed to delete backup receiver: " + filename, "err", err.Error())
				}
			} else {
				level.Debug(c.logger).Log("msg", "Backup receiver does not exist")
			}
		}
	} else if typ == "routes" {
		for k := range configmapObj.Data {
			filename := configmapObj.Namespace + "-" + configmapObj.Name + "-" + k
			level.Debug(c.logger).Log("mag", "Delete backup route if it is existed")
			if _, err := os.Stat(c.a.ConfigPath + "/backup-routes/" + filename); !os.IsNotExist(err) {
				err := os.Remove(c.a.ConfigPath + "/backup-routes/" + filename)
				if err != nil {
					level.Error(c.logger).Log("msg", "Failed to delete backup route: " + filename, "err", err.Error())
				}
			} else {
				level.Debug(c.logger).Log("msg", "Backup route does not exist")
			}
		}
	} else if typ == "inhibit rule" {
		for k := range configmapObj.Data {
			filename := configmapObj.Namespace + "-" + configmapObj.Name + "-" + k
			level.Debug(c.logger).Log("mag", "Delete backup inhibit rule if it is existed")
			if _, err := os.Stat(c.a.ConfigPath + "/backup-inhibit-rules/" + filename); !os.IsNotExist(err) {
				err := os.Remove(c.a.ConfigPath + "/backup-inhibit-rules/" + filename)
				if err != nil {
					level.Error(c.logger).Log("msg", "Failed to delete backup inhibit rule: " + filename, "err", err.Error())
				}
			} else {
				level.Debug(c.logger).Log("msg", "Backup inhibit rule does not exist")
			}
		}
	}
}

// are two configmaps same
func noDifference(newConfigMap *v1.ConfigMap, oldConfigMap *v1.ConfigMap) bool {
	if len(newConfigMap.Data) != len(oldConfigMap.Data) {
		return false
	}
	for k, v := range newConfigMap.Data {
		if v != oldConfigMap.Data[k]{
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
func (c *Controller)copyFile(sourceFile string, targetFile string) {
	source, err := os.Open(sourceFile)
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to read file", "file", sourceFile, "err", err.Error())
	}
	defer source.Close()

	dest, err := os.Create(targetFile)
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to create file", "file", sourceFile, "err", err.Error())
	}
	defer dest.Close()

	_, err = io.Copy(dest, source)
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to copy file", "file", sourceFile, "err", err.Error())
	}
}

func (c *Controller) checkBackupInhibitRules() {
	inhibitRulePath, err := filepath.Glob(c.a.ConfigPath + "/backup-inhibit-rules/*")
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to read backup inhibit rules", "err", err.Error())
	}

	routes       := c.readConfigs("routes")
	receivers    := c.readConfigs("receivers")
	inhibitRules := c.readConfigs("inhibit-rules")

	var alertmanagerConfig alertmanager.AlertmanagerConfig
	alertmanagerConfig.Routes = strings.Replace(routes,"\n", "\n  ", -1)
	alertmanagerConfig.Receivers = receivers

	configTemplate, err := ioutil.ReadFile(c.a.ConfigTemplate)
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to read template: " + c.a.ConfigTemplate, "err", err.Error())
	}

	t, err := template.New("alertmanager.yml").Parse(string(configTemplate))
	if err != nil {
		level.Error(c.logger).Log("msg", "Failed to parse template" , "err", err.Error())
	}

	for _, inhibitRuleFile := range inhibitRulePath {
		inhibitRule, err := ioutil.ReadFile(inhibitRuleFile)
		if err != nil {
			level.Error(c.logger).Log("msg", "Failed to read inhibit rule: " + inhibitRuleFile, "err", err.Error())
		}
		newInhibitRules := inhibitRules + string(inhibitRule)

		alertmanagerConfig.InhibitRules = newInhibitRules
		var tpl bytes.Buffer
		err = t.Execute(&tpl, alertmanagerConfig)
		_, configErr := alcf.Load(tpl.String())
		if configErr == nil{
			c.copyFile(inhibitRuleFile, c.a.ConfigPath + "/inhibit-rules/" + filepath.Base(inhibitRuleFile))
			err = os.Remove(inhibitRuleFile)
			if err != nil {
				level.Error(c.logger).Log("msg", "Failed to delete inhibitRule: " + inhibitRuleFile, "err", err.Error())
			}
			level.Debug(c.logger).Log("msg", "Inhibit rule is available", "inhibitRule", inhibitRuleFile)
		} else {
			level.Debug(c.logger).Log("msg", "Inhibit rule is unavailable", "inhibitRule", inhibitRuleFile, "err", configErr)
		}
	}

}
