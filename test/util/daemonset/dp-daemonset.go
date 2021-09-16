package util

import (
	"context"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	appsclient "k8s.io/client-go/kubernetes/typed/apps/v1"
)

func createDsYaml(dsName, dsNamespace, imageName, imageTag string) string {
	return `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ` + dsName + `
  namespace: ` + dsNamespace + `
  labels:
    tier: node
    app: sriovdp
spec:
  selector:
    matchLabels:
      name: sriov-device-plugin
  template:
    metadata:
      labels:
        name: sriov-device-plugin
        tier: node
        app: sriovdp
    spec:
      hostNetwork: true
      nodeSelector:
        kubernetes.io/arch: amd64
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      serviceAccountName: sriov-device-plugin
      containers:
      - name: kube-sriovdp
        image: ` + imageName + ":" + imageTag + `
        imagePullPolicy: Always
        args:
        - --log-dir=sriovdp
        - --log-level=10
        securityContext:
          privileged: true
        volumeMounts:
        - name: devicesock
          mountPath: /var/lib/kubelet/
          readOnly: false
        - name: log
          mountPath: /var/log
        - name: config-volume
          mountPath: /etc/pcidp
        - name: device-info
          mountPath: /var/run/k8s.cni.cncf.io/devinfo/dp
      volumes:
        - name: devicesock
          hostPath:
            path: /var/lib/kubelet/
        - name: log
          hostPath:
            path: /var/log
        - name: device-info
          hostPath:
            path: /var/run/k8s.cni.cncf.io/devinfo/dp
            type: DirectoryOrCreate
        - name: config-volume
          configMap:
            name: sriovdp-config
            items:
            - key: config.json
              path: config.json`
}

// CreateDpDaemonset Creates device plugin's daemonset
func CreateDpDaemonset(ac *appsclient.AppsV1Client, dsName, dsNamespace, imageName, imageTag string) (*appsv1.DaemonSet, error) {
	dsYaml := createDsYaml(dsName, dsNamespace, imageName, imageTag)

	reader := strings.NewReader(dsYaml)
	ds := &appsv1.DaemonSet{}
	err := yaml.NewYAMLOrJSONDecoder(reader, len(dsYaml)).Decode(ds)

	if err != nil {
		return nil, err
	}

	dsObj, err := ac.DaemonSets(dsNamespace).Create(context.TODO(), ds, metav1.CreateOptions{})
	return dsObj, err
}

// Delete deletes specified by name and namespace daemonset
func Delete(ac *appsclient.AppsV1Client, name, namespace string) error {
	return ac.DaemonSets(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}
