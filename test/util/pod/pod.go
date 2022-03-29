package util

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	util "github.com/k8snetworkplumbingwg/sriov-cni/test/util"
)

// CreateRunningPod create a pod and wait until it is running
func CreateRunningPod(ci coreclient.CoreV1Interface, pod *corev1.Pod, timeout, interval time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	pod, err := ci.Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	err = WaitForPodStateRunning(ci, pod.ObjectMeta.Name, pod.ObjectMeta.Namespace, timeout, interval)
	if err != nil {
		return err
	}
	return nil
}

// TryToCreateRunningPod - try to create running POD - expect failure or success depening on timeout // util.ShortTimeout
func TryToCreateRunningPod(core coreclient.CoreV1Interface, podName, namespace, networkName, networkResourceName string, timeout time.Duration) (*corev1.Pod, error) {
	podObj := GetPodDefinition(podName, namespace)
	podObj = AddPodNetworkAnnotation(podObj, "k8s.v1.cni.cncf.io/networks", networkName)
	podObj = AddPodNetworkResourcesAndLimits(podObj, 0, 1, 1, networkResourceName)
	err := CreateRunningPod(core, podObj, timeout, util.RetryInterval)

	return podObj, err
}

// CreateListOfPods - create specified number of PODs and returns each inside slice
func CreateListOfPods(core coreclient.CoreV1Interface, number int, namePrefix, namespace, networkName, networkResourceName string) ([]*corev1.Pod, error) {
	var podList []*corev1.Pod
	var err error

	for index := 0; index < number; index++ {
		podLoop, err := TryToCreateRunningPod(core, fmt.Sprintf("%s-%s", namePrefix, strconv.Itoa(index)), namespace, networkName, networkResourceName, util.Timeout)
		podList = append(podList, podLoop)
		if err != nil {
			return podList, err
		}
	}

	return podList, err
}

// WaitForPodStateRunning waits for pod to enter running state
func WaitForPodStateRunning(core coreclient.CoreV1Interface, podName, ns string, timeout, interval time.Duration) error {
	return wait.PollImmediate(interval, timeout, func() (done bool, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		pod, err := core.Pods(ns).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			if strings.Contains(err.Error(), fmt.Sprintf("pods \"%s\" not found", podName)) {
				return false, nil
			}

			return false, err
		}

		switch pod.Status.Phase {
		case corev1.PodRunning:
			return true, nil
		case corev1.PodFailed, corev1.PodSucceeded:
			return false, errors.New("pod failed or succeeded but is not running")
		}
		return false, nil
	})
}

// WaitForPodStateRunningLabel - wait for POD with specific label to be created and in run state
func WaitForPodStateRunningLabel(core coreclient.CoreV1Interface, podLabel, ns string, timeout, interval time.Duration) error {
	return wait.PollImmediate(interval, timeout, func() (done bool, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		pods, err := core.Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: podLabel})
		if err != nil {
			return false, err
		}

		if len(pods.Items) == 0 {
			return false, nil
		}

		for _, pod := range pods.Items {
			return true, WaitForPodStateRunning(core, pod.Name, pod.Namespace, timeout, interval)
		}

		return false, errors.New("POD with specified label [" + podLabel + "] was not found.")
	})
}

// UpdatePodInfo will get the current pod state and return it
func UpdatePodInfo(ci coreclient.CoreV1Interface, pod *corev1.Pod, timeout time.Duration) (*corev1.Pod, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	pod, err := ci.Pods(pod.Namespace).Get(ctx, pod.ObjectMeta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return pod, nil
}

// GetPodDefinition returns POD definition
func GetPodDefinition(podName, ns string) *corev1.Pod {
	var graceTime int64 = 0
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &graceTime,
			Containers: []corev1.Container{
				{
					Name:    "test",
					Image:   "praqma/network-multitool:alpine-extra",
					Command: []string{"/bin/sh", "-c", "sleep INF"},
				},
			},
		},
	}
}

// AddPodNetworkAnnotation adds to given pod a key, value pair to the annotations map
func AddPodNetworkAnnotation(pod *corev1.Pod, key, value string) *corev1.Pod {
	if nil == pod.Annotations {
		pod.Annotations = make(map[string]string)
	}

	pod.Annotations[key] = value

	return pod
}

// AddPodNetworkResourcesAndLimits add resources and limits to selected container
func AddPodNetworkResourcesAndLimits(pod *corev1.Pod, container, limit, request int64, key string) *corev1.Pod {
	if nil == pod.Spec.Containers[container].Resources.Limits {
		pod.Spec.Containers[container].Resources.Limits = make(map[corev1.ResourceName]resource.Quantity)
	}

	if nil == pod.Spec.Containers[container].Resources.Requests {
		pod.Spec.Containers[container].Resources.Requests = make(map[corev1.ResourceName]resource.Quantity)
	}

	pod.Spec.Containers[container].Resources.Limits[corev1.ResourceName(key)] = *resource.NewQuantity(limit, resource.DecimalSI)
	pod.Spec.Containers[container].Resources.Requests[corev1.ResourceName(key)] = *resource.NewQuantity(request, resource.DecimalSI)

	return pod
}

// AddToPodDefinitionCPULimits adds CPU limits and requests to the definition of POD
func AddToPodDefinitionCPULimits(pod *corev1.Pod, containerNumber, cpuNumber int64) *corev1.Pod {
	if nil == pod.Spec.Containers[containerNumber].Resources.Limits {
		pod.Spec.Containers[containerNumber].Resources.Limits = make(map[corev1.ResourceName]resource.Quantity)
	}

	if nil == pod.Spec.Containers[containerNumber].Resources.Requests {
		pod.Spec.Containers[containerNumber].Resources.Requests = make(map[corev1.ResourceName]resource.Quantity)
	}

	pod.Spec.Containers[containerNumber].Resources.Limits["cpu"] = *resource.NewQuantity(cpuNumber, resource.DecimalSI)
	pod.Spec.Containers[containerNumber].Resources.Requests["cpu"] = *resource.NewQuantity(cpuNumber, resource.DecimalSI)

	return pod
}

// AddToPodDefinitionHugePages1Gi adds Hugepages 1Gi limits and requirements to the POD spec
func AddToPodDefinitionHugePages1Gi(pod *corev1.Pod, containerNumber, amountLimit, amountRequest int64) *corev1.Pod {
	if nil == pod.Spec.Containers[containerNumber].Resources.Limits {
		pod.Spec.Containers[containerNumber].Resources.Limits = make(map[corev1.ResourceName]resource.Quantity)
	}

	if nil == pod.Spec.Containers[containerNumber].Resources.Requests {
		pod.Spec.Containers[containerNumber].Resources.Requests = make(map[corev1.ResourceName]resource.Quantity)
	}

	pod.Spec.Containers[containerNumber].Resources.Limits["hugepages-1Gi"] = *resource.NewQuantity(amountLimit*1024*1024*1024, resource.BinarySI)
	pod.Spec.Containers[containerNumber].Resources.Requests["hugepages-1Gi"] = *resource.NewQuantity(amountRequest*1024*1024*1024, resource.BinarySI)

	return pod
}

// Delete will delete a pod
func Delete(ci coreclient.CoreV1Interface, pod *corev1.Pod, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := ci.Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	pod = nil
	return err
}

// DeleteList will delete list of pods
func DeleteList(ci coreclient.CoreV1Interface, podList []*corev1.Pod, timeout time.Duration) error {
	var isError bool
	for _, podItem := range podList {
		err := Delete(ci, podItem, timeout)
		if err != nil {
			isError = true
		}
	}

	if isError {
		return errors.New("some POD returns error during deletions")
	}

	return nil
}

// GetPodEventsList returns list of events for selected POD
func GetPodEventsList(ci coreclient.CoreV1Interface, pod *corev1.Pod, timeout time.Duration) (*corev1.EventList, error) {
	return GetPodEventsListRaw(ci, pod.Name, pod.Namespace, timeout)
}

// GetPodEventsListRaw returns list of events for given podName in PodNamesapce
func GetPodEventsListRaw(ci coreclient.CoreV1Interface, podName, podNamespace string, timeout time.Duration) (*corev1.EventList, error) {
	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.namespace=%s", podName, podNamespace),
	}

	events, err := ci.Events(podNamespace).List(context.TODO(), listOptions)
	if len(events.Items) == 0 && err == nil {
		err = errors.New("no suitable pods found")
	}

	return events, err
}

// WaitForPodRecreation waits for pod selected by selector to get deleted and recreated with daemonset
func WaitForPodRecreation(core coreclient.CoreV1Interface, oldPodName, ns string, selectors map[string]string, timeout, interval time.Duration) error {
	if err := waitForPodDeletion(core, oldPodName, ns, timeout, interval); err != nil {
		return err
	}
	pods, _ := GetPodWithSelectors(core, &ns, selectors)
	return waitForPodStateRunning(core, pods.Items[0].Name, pods.Items[0].Namespace, timeout, interval)
}

//WaitForPodStateRunning waits for pod to enter running state
func waitForPodStateRunning(core coreclient.CoreV1Interface, podName, ns string, timeout, interval time.Duration) error {
	return wait.PollImmediate(interval, timeout, func() (done bool, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		pod, err := core.Pods(ns).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		switch pod.Status.Phase {
		case corev1.PodRunning:
			return true, nil
		case corev1.PodFailed, corev1.PodSucceeded:
			return false, errors.New("pod failed or succeeded but is not running")
		}
		return false, nil
	})
}

//waitForPodDeletion waits for pod to be deleted
func waitForPodDeletion(core coreclient.CoreV1Interface, podName, ns string, timeout, interval time.Duration) error {
	result := wait.PollImmediate(interval, timeout, func() (done bool, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		_, err = core.Pods(ns).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return false, nil
	})

	if result != nil {
		return nil
	}

	return errors.New("could not delete pod")
}

// GetPodWithSelectors returns list of pod that matches selectors
func GetPodWithSelectors(ci coreclient.CoreV1Interface, namespace *string, selectors map[string]string) (*corev1.PodList, error) {
	labelSelector := prepareSelectors(selectors)
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}
	pods, err := ci.Pods(*namespace).List(context.TODO(), listOptions)
	if len(pods.Items) == 0 && err == nil {
		err = errors.New("no suitable pods found")
	}
	return pods, err
}

// WaitForDpResourceUpdate waits for DP to send update
func WaitForDpResourceUpdate(ci coreclient.CoreV1Interface, pod corev1.Pod, timeout, interval time.Duration) (bool, error) {
	isUpdated := false
	var err error
	retries := int(timeout / interval)
	for i := 0; i < retries; i++ {
		isUpdated, err = checkForDpResourceUpdate(ci, pod)
		if isUpdated {
			time.Sleep(15 * time.Second)
			break
		}
		time.Sleep(interval)
	}
	return isUpdated, err
}

//checkForDpResourceUpdate checks if device plugin's resource list has been updated
func checkForDpResourceUpdate(ci coreclient.CoreV1Interface, pod corev1.Pod) (bool, error) {
	isUpdated := false
	res := ci.Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})

	logsStream, err := res.Stream(context.TODO())
	if err != nil {
		return isUpdated, err
	}
	defer logsStream.Close()

	tmpBuf := new(bytes.Buffer)
	_, err = io.Copy(tmpBuf, logsStream)
	if err != nil {
		return isUpdated, err
	}

	logStr := tmpBuf.String()

	if strings.Contains(logStr, "send devices") {
		isUpdated = true
	}

	return isUpdated, nil
}

//prepareSelectors perpares pod selectors to be used
func prepareSelectors(labels map[string]string) string {
	var result string
	index := 0
	for key, value := range labels {
		result += key + "=" + value
		if index < (len(labels) - 1) {
			result += ","
		}
		index++
	}
	return result
}

// ExecuteCommand execute command on the POD
// :param core - core V1 Interface
// :param config - configuration used to establish REST connection with K8s node
// :param podName - POD name on which command has to be executed
// :param ns - namespace in which POD exists
// :param containerName - container name on which command should be executed
// :param command - command to be executed on POD
// :return string output of the command (stdout)
// 	       string output of the command (stderr)
//         error Error object or when everthing is correct nil
func ExecuteCommand(core coreclient.CoreV1Interface, config *restclient.Config, podName, ns, containerName, command string) (string, string, error) {
	shellCommand := []string{"/bin/sh", "-c", command}
	request := core.RESTClient().Post().Resource("pods").Name(podName).Namespace(ns).SubResource("exec")
	options := &corev1.PodExecOptions{
		Command:   shellCommand,
		Container: containerName,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}

	request.VersionedParams(options, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", request.URL())
	if nil != err {
		return "", "", err
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	return stdout.String(), stderr.String(), err
}
