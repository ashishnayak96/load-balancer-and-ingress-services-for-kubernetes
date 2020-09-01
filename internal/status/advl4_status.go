/*
 * Copyright 2019-2020 VMware, Inc.
 * All Rights Reserved.
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*   http://www.apache.org/licenses/LICENSE-2.0
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*/

package status

import (
	"errors"
	"strings"

	advl4v1alpha1pre1 "github.com/vmware-tanzu/service-apis/apis/v1alpha1pre1"
	"github.com/vmware/load-balancer-and-ingress-services-for-kubernetes/internal/lib"
	"github.com/vmware/load-balancer-and-ingress-services-for-kubernetes/pkg/utils"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Type: specific condition enums must be part of independent update functions
type UpdateGatewayStatusOptions struct {
	Status             core.ConditionStatus // defaults to True
	Message            string               // extended condition message
	Reason             string               // reason for transition
	LastTransitionTime metav1.Time          // send in time of function call
}

// TODO: handle bulk during bootup
func UpdateGatewayStatusAddress(options []UpdateStatusOptions, bulk bool) {
	for _, option := range options {
		gatewayNSName := strings.Split(option.ServiceMetadata.Gateway, "/")
		gw, err := lib.GetAdvL4Clientset().NetworkingV1alpha1pre1().Gateways(gatewayNSName[0]).Get(gatewayNSName[1], metav1.GetOptions{})
		if err != nil {
			utils.AviLog.Infof("key: %s, msg: unable to find gateway object %s", option.Key, option.ServiceMetadata.Gateway)
		}

		// assuming 1 IP per gateway
		gwStatus := gw.Status
		gwStatus.Addresses = []advl4v1alpha1pre1.GatewayAddress{{
			Value: option.Vip,
			Type:  advl4v1alpha1pre1.IPAddressType,
		}}
		updateGatewayStatusObject(gw, &gwStatus)
	}

	return
}

// supported GatewayConditionTypes
// InvalidListeners, InvalidAddress, *Serviceable
func UpdateGatewayStatusGWCondition(gw *advl4v1alpha1pre1.Gateway, gwConditionType advl4v1alpha1pre1.GatewayConditionType, updateStatus *UpdateGatewayStatusOptions) {
	gwStatus := gw.Status
	for _, condition := range gwStatus.Conditions {
		if condition.Type == gwConditionType {
			condition.Status = updateStatus.Status
			condition.Message = updateStatus.Message
			condition.Reason = updateStatus.Reason
			break
		}
	}

	updateGatewayStatusObject(gw, &gwStatus)
}

// supported ListenerConditionType
// PortConflict, UnsupportedProtocol, InvalidRoutes, UnsupportedProtocol, *Serviceable
func UpdateGatewayStatusListenerConditions(gw *advl4v1alpha1pre1.Gateway, port string, listenerConditionType advl4v1alpha1pre1.ListenerConditionType, updateStatus *UpdateGatewayStatusOptions) {
	gwStatus := gw.Status
	for _, condition := range gwStatus.Listeners {
		if condition.Port == port {
			for _, portCondition := range condition.Conditions {
				if portCondition.Type == listenerConditionType {
					portCondition.Status = updateStatus.Status
					portCondition.Message = updateStatus.Message
					portCondition.Reason = updateStatus.Reason
					break
				}
			}
			break
		}
	}

	updateGatewayStatusObject(gw, &gwStatus)
}

func updateGatewayStatusObject(gw *advl4v1alpha1pre1.Gateway, updateStatus *advl4v1alpha1pre1.GatewayStatus, retryNum ...int) error {
	retry := 0
	if len(retryNum) > 0 {
		retry = retryNum[0]
		if retry >= 4 {
			return errors.New("msg: UpdateGatewayStatus retried 5 times, aborting")
		}
	}

	gw.Status = *updateStatus
	_, err := lib.GetAdvL4Clientset().NetworkingV1alpha1pre1().Gateways(gw.Namespace).UpdateStatus(gw)
	if err != nil {
		utils.AviLog.Errorf("msg: %d there was an error in updating the gateway status: %+v", retry, err)
		updatedGW, err := lib.GetAdvL4Clientset().NetworkingV1alpha1pre1().Gateways(gw.Namespace).Get(gw.Name, metav1.GetOptions{})
		if err != nil {
			utils.AviLog.Warnf("gateway not found %v", err)
			return err
		}
		return updateGatewayStatusObject(updatedGW, updateStatus, retry+1)
	}

	utils.AviLog.Infof("msg: Successfully updated the gateway %s/%s status %+v", gw.Namespace, gw.Name, utils.Stringify(updateStatus))
	return nil
}