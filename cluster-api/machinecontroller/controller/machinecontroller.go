package controller

import (
	"context"

	"github.com/golang/glog"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	machinesv1 "k8s.io/kube-deploy/cluster-api/api/machines/v1alpha1"
	"k8s.io/kube-deploy/cluster-api/cloud"
)

type MachineController struct {
	config     *Configuration
	restClient *rest.RESTClient
	actuator cloud.MachineActuator
}

func NewMachineController(config *Configuration) *MachineController{
	restClient, _, err := restClient(config.Kubeconfig)
	if err != nil {
		glog.Fatalf("error creating rest client: %v", err)
	}

	masterIP, err := host(config.Kubeconfig)
	if err != nil {
		glog.Fatalf("error getting master IP from rest client: %v", err)
	}

	// Determine cloud type from cluster CRD when available
	actuator, err := newMachineActuator(config.Cloud, config.KubeadmToken, masterIP)
	if err != nil {
		glog.Fatalf("error creating machine actuator: %v", err)
	}

	return &MachineController{
			config:config,
			restClient:restClient,
		actuator:actuator,
		}
}

func (c *MachineController) Run () error {
	glog.Infof("Running ...")

	// Run leader election

	return c.run(context.Background())
}

func (c *MachineController) run(ctx context.Context) error {
	source := cache.NewListWatchFromClient(c.restClient, "machines", apiv1.NamespaceAll, fields.Everything())

	_, informer := cache.NewInformer(
		source,
		&machinesv1.Machine{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.onAdd,
			UpdateFunc: c.onUpdate,
			DeleteFunc: c.onDelete,
		},
	)

	informer.Run(ctx.Done())
	return nil
}

func (c *MachineController) onAdd(obj interface{}) {
	machine := obj.(*machinesv1.Machine)
	glog.Infof("object created: %s\n", machine.ObjectMeta.Name)
	err := c.create(machine)
	if err != nil {
		glog.Errorf("create machine %s failed: %v", machine.ObjectMeta.Name, err)
	}

}

func (c *MachineController) onUpdate(oldObj, newObj interface{}) {
	oldMachine := oldObj.(*machinesv1.Machine)
	newMachine := newObj.(*machinesv1.Machine)
	glog.Infof("object updated: %s\n", oldMachine.ObjectMeta.Name)
	glog.Infof("  old k8s version: %s, new: %s\n", oldMachine.Spec.Versions.Kubelet, newMachine.Spec.Versions.Kubelet)

	//TODO: Only trigger delete and recreate if the change to the object warrants such.
	// eg. linking and unlinking a node to the machine object does not warrant a re-create
	err := c.delete(oldMachine)
	if err != nil {
		glog.Errorf("delete machine %s for update failed: %v", oldMachine.ObjectMeta.Name, err)
		return
	}
	err = c.create(newMachine)
	if err != nil {
		glog.Errorf("create machine %s for update failed: %v", newMachine.ObjectMeta.Name, err)
	}
}

func (c *MachineController) onDelete(obj interface{}) {
	machine := obj.(*machinesv1.Machine)
	glog.Infof("object deleted: %s\n", machine.ObjectMeta.Name)
	err := c.delete(machine)
	if err != nil {
		glog.Errorf("delete machine %s failed: %v", machine.ObjectMeta.Name, err)
	}
}


func (c *MachineController) create(machine *machinesv1.Machine) error {
	//TODO: check if the actual machine does not already exist
	return c.actuator.Create(machine)
	//TODO: wait for machine to become a node
	//TODO: link node to machine CRD
}

func (c *MachineController) delete(machine *machinesv1.Machine) error {
	//TODO: check if the actual machine does not exist
	//TODO: delink node from machine CRD
	//TODO: remove (and possibly drain) node
	return c.actuator.Delete(machine)
}