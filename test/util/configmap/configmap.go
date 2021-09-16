package util

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
)

// GetConfigMap returns config map
func GetConfigMap(confgiMapName, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      confgiMapName,
			Namespace: namespace,
		},
	}
}

// CreateDevicePluginCM creates config map object for device plugin
func CreateDevicePluginCM(configMapName, namespace, configMapDpValue string) *corev1.ConfigMap {
	configMapDp := GetConfigMap(configMapName, namespace)
	configMapDp = AddData(configMapDp, "config.json", configMapDpValue)

	return configMapDp
}

// AddData adds config map data (key, value) pair
func AddData(configMap *corev1.ConfigMap, dataKey, dataValue string) *corev1.ConfigMap {
	if nil == configMap.Data {
		configMap.Data = make(map[string]string)
	}

	configMap.Data[dataKey] = dataValue

	return configMap
}

// Apply apply config map definition to the cluster
func Apply(ci coreclient.CoreV1Interface, configMap *corev1.ConfigMap, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	defer cancel()
	_, err := ci.ConfigMaps(configMap.Namespace).Create(ctx, configMap, metav1.CreateOptions{})

	if err != nil {
		return err
	}

	return nil
}

// Delete deletes config map
func Delete(ci coreclient.CoreV1Interface, name, ns string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := ci.ConfigMaps(ns).Delete(ctx, name, metav1.DeleteOptions{})
	return err
}
