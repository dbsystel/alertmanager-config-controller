# 0.2.5 / 2022-02-23
* [BUGFIX] Avoid panic: assignment to entry in nil map with empty config maps for routes

# 0.2.4 / 2019-06-28
 * [BUGFIX] Fixed incorrect indentation in configmap examples 

# 0.2.3 / 2019-06-28
 * [BUGFIX] Disabled for now bodyclose checking in linter
 * [BUGFIX] Fixed addContinueIfNotExist handling route

# 0.2.2 / 2019-06-13
 * [ENHANCEMENT] Updated ci release handling

# 0.2.1 / 2019-06-11
 * [BUGFIX] Fixed linter errors

## 0.2.0 / 2019-06-11
* [ENHANCEMENT] Adds example helm installation charts
* [ENHANCEMENT] Adds test, lint and build setup
* [ENHANCEMENT] Adds automatic releasing with goreleaser and travis

## 0.1.3 / 2019-05-14
* [ENHANCEMENT] Automatically add `continue: true` to the first level of route
* [CHANGE] The controller will delete the config, if the annotation in configmap is switched from `true` to `false`
* [CHANGE] Deleted glide dep management; updated to go version 1.12; using now go.mod

## 0.1.2 / 2019-04-18
* [BUGFIX] Sometimes if there is a route configuration with undefined receiver, all configurations which are made after this one will be ignored.

## 0.1.1 / 2019-03-13
* [BUGFIX] Update of alertmanager.yml doesn't work properly 

## 0.1.0 / 2019-03-07
* [INITIAL] Initial commit of alertmanager-config-controller
