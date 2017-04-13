package check

import (
	"fmt"
	"net/http"
	"strings"
	"io/ioutil"
	flag "github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/api"
	"reflect"
	"encoding/json"
	"time"
	"github.com/hootsuite/sens8/util"
)

type HsHealthCheck struct {
	BaseCheck
	url      *string
	pod      *api.Pod
	resource interface{}
}

//NewHsHealthCheck creates a new deployment health check
func NewHsHealthCheck(config CheckConfig) (Check, error) {
	h := HsHealthCheck{}
	h.Config = config

	// process flags
	commandLine := flag.NewFlagSet(config.Id, flag.ContinueOnError)
	h.url = commandLine.String("url", "", "url to query. :::POD_IP::: gets replace with the pod's IP. :::HOST_IP::: gets replaced with the pod's host ip. :::CUSTER_IP::: gets replaced by the service's ip")
	if err := commandLine.Parse(config.Argv[1:]); err != nil {
		return &h, nil
	}
	if *h.url == "" {
		fmt.Errorf("--url cannot be empty")
	}
	return &h, nil
}

func (h *HsHealthCheck) Update(resource interface{}) {
	h.pod = resource.(*api.Pod)
	h.resource = resource
}

func (h *HsHealthCheck) Execute() (CheckResult, error) {
	start := time.Now()
	result := NewCheckResultFromConfig(h.Config)
	url := *h.url

	// @todo cast based on resource "apiVersion"
	t := reflect.TypeOf(h.resource).String()
	t = t[strings.LastIndex(t, ".") + 1:]
	switch t {
	case "Pod":
		pod := h.resource.(*api.Pod)
		url = strings.Replace(url, ":::POD_IP:::", pod.Status.PodIP, -1)
		url = strings.Replace(url, ":::HOST_IP:::", pod.Status.HostIP, -1)
	case "Service":
		service := h.resource.(*api.Service)
		url = strings.Replace(url, ":::CUSTER_IP:::", service.Spec.ClusterIP, -1)
	default:
		return result, fmt.Errorf("resource type is unknown")
	}

	// make http request
	// @todo - add request timeout. make this configurable
	resp, err := http.Get(url)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}

	var status []interface{}
	err = json.Unmarshal(buf, &status)

	// determine status code
	if err != nil || len(status) == 0 {
		result.Status = CRITICAL
	} else {
		switch status[0] {
		case "OK": result.Status = OK
		case "WARN": result.Status = WARN
		case "CRIT": result.Status = CRITICAL
		}
	}

	// limit the output size to sensu
	if len(buf) > 1024 {
		buf = buf[:1024]
	}
	result.Output = string(buf)
	result.Duration = util.SecondsSince(start)

	return result, nil
}

// register factory
func init() {
	RegisterCheck("hs_healthcheck", NewHsHealthCheck, []string{"pod"})
}
