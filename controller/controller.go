/**
 * Copyright 2017 IBM Corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package controller

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"github.com/IBM/ubiquity/utils/logs"


	"bytes"
	k8sresources "github.com/IBM/ubiquity-k8s/resources"
	"github.com/IBM/ubiquity/remote"
	"github.com/IBM/ubiquity/resources"
	"github.com/IBM/ubiquity/utils"
	"path/filepath"
	"github.com/IBM/ubiquity/remote/mounter"
	"github.com/nightlyone/lockfile"
	"time"
)

//Controller this is a structure that controls volume management
type Controller struct {
	Client resources.StorageClient
	exec   utils.Executor
	logger logs.Logger
	legacyLogger *log.Logger
	config resources.UbiquityPluginConfig
	mounterPerBackend map[string]resources.Mounter
	unmountFlock       lockfile.Lockfile
}

//NewController allows to instantiate a controller
func NewController(logger *log.Logger, config resources.UbiquityPluginConfig) (*Controller, error) {
	unmountFlock, err := lockfile.New(filepath.Join(os.TempDir(), "ubiquity.unmount.lock"))
	if err != nil {
		panic(err)
	}

	remoteClient, err := remote.NewRemoteClientSecure(logger, config)
	if err != nil {
		return nil, err
	}
	return &Controller{
		logger: logs.GetLogger(),
		legacyLogger: logger,
		Client: remoteClient,
		exec: utils.NewExecutor(),
		config: config,
		mounterPerBackend: make(map[string]resources.Mounter),
		unmountFlock: unmountFlock}, nil
}

//NewControllerWithClient is made for unit testing purposes where we can pass a fake client
func NewControllerWithClient(logger *log.Logger, client resources.StorageClient, exec utils.Executor) *Controller {
	unmountFlock, err := lockfile.New(filepath.Join(os.TempDir(), "ubiquity.unmount.lock"))
	if err != nil {
		panic(err)
	}

	utils.NewExecutor()
	return &Controller{logger: logs.GetLogger(), legacyLogger: logger, Client: client, exec: exec, unmountFlock: unmountFlock}
}



//Init method is to initialize the k8sresourcesvolume
func (c *Controller) Init(config resources.UbiquityPluginConfig) k8sresources.FlexVolumeResponse {
	defer c.logger.Trace(logs.DEBUG)()

	response := k8sresources.FlexVolumeResponse{
		Status:  "Success",
		Message: "Plugin init successfully",
	}

	c.logger.Debug("", logs.Args{{"response", response}})
	return response
}

//TestUbiquity method is to test connectivity to ubiquity
func (c *Controller) TestUbiquity(config resources.UbiquityPluginConfig) k8sresources.FlexVolumeResponse {
	defer c.logger.Trace(logs.DEBUG)()
	var response k8sresources.FlexVolumeResponse

	activateRequest := resources.ActivateRequest{Backends: config.Backends}
	c.logger.Debug("", logs.Args{{"request", activateRequest}})

	err := c.doActivate(activateRequest)
	if err != nil {
		response = k8sresources.FlexVolumeResponse{
			Status:  "Failure",
			Message: fmt.Sprintf("Test ubiquity failed %#v ", err),
		}
	} else {
		response = k8sresources.FlexVolumeResponse{
			Status:  "Success",
			Message: "Test ubiquity successfully",
		}
	}

	c.logger.Debug("", logs.Args{{"response", response}})
	return response
}

//Attach method attaches a volume to a host
func (c *Controller) Attach(attachRequest k8sresources.FlexVolumeAttachRequest) k8sresources.FlexVolumeResponse {
	defer c.logger.Trace(logs.DEBUG)()
	var response k8sresources.FlexVolumeResponse
	c.logger.Debug("", logs.Args{{"request", attachRequest}})

	err := c.doAttach(attachRequest)
	if err != nil {
		msg := fmt.Sprintf("Failed to attach volume [%s], Error: %#v", attachRequest.Name, err)
		response = k8sresources.FlexVolumeResponse{
			Status:  "Failure",
			Message: msg,
		}
	} else {
		response = k8sresources.FlexVolumeResponse{
			Status: "Success",
		}
	}

	c.logger.Debug("", logs.Args{{"response", response}})
	return response
}


//GetVolumeName checks if volume is attached
func (c *Controller) GetVolumeName(getVolumeNameRequest k8sresources.FlexVolumeGetVolumeNameRequest) k8sresources.FlexVolumeResponse {
	defer c.logger.Trace(logs.DEBUG)()
	var response k8sresources.FlexVolumeResponse
	c.logger.Debug("", logs.Args{{"request", getVolumeNameRequest}})

	response = k8sresources.FlexVolumeResponse{
		Status: "Not supported",
	}

	c.logger.Debug("", logs.Args{{"response", response}})
	return response
}

//WaitForAttach Waits for a volume to get attached to the node
func (c *Controller) WaitForAttach(waitForAttachRequest k8sresources.FlexVolumeWaitForAttachRequest) k8sresources.FlexVolumeResponse {
	defer c.logger.Trace(logs.DEBUG)()
	var response k8sresources.FlexVolumeResponse
	c.logger.Debug("", logs.Args{{"request", waitForAttachRequest}})

	response = k8sresources.FlexVolumeResponse{
		Status: "Not supported",
	}

	c.logger.Debug("", logs.Args{{"response", response}})
	return response
}

//IsAttached checks if volume is attached
func (c *Controller) IsAttached(isAttachedRequest k8sresources.FlexVolumeIsAttachedRequest) k8sresources.FlexVolumeResponse {
	defer c.logger.Trace(logs.DEBUG)()
	var response k8sresources.FlexVolumeResponse
	c.logger.Debug("", logs.Args{{"request", isAttachedRequest}})

	isAttached, err := c.doIsAttached(isAttachedRequest)
	if err != nil {
		msg := fmt.Sprintf("Failed to check IsAttached volume [%s], Error: %#v", isAttachedRequest.Name, err)
		response = k8sresources.FlexVolumeResponse{
			Status:  "Failure",
			Message: msg,
		}
	} else {
		response = k8sresources.FlexVolumeResponse{
			Status: "Success",
			Attached: isAttached,
		}
	}

	c.logger.Debug("", logs.Args{{"response", response}})
	return response
}

//Detach detaches the volume/ fileset from the pod
func (c *Controller) Detach(detachRequest k8sresources.FlexVolumeDetachRequest) k8sresources.FlexVolumeResponse {
	defer c.logger.Trace(logs.DEBUG)()
	var response k8sresources.FlexVolumeResponse
	c.logger.Debug("", logs.Args{{"request", detachRequest}})

	if detachRequest.Version == k8sresources.KubernetesVersion_1_5 {
		c.logger.Debug("legacy detach (skipping)")
		response = k8sresources.FlexVolumeResponse{
			Status: "Success",
		}
	} else {
		err := c.doDetach(detachRequest, true)
		if err != nil {
			msg := fmt.Sprintf(
				"Failed to detach volume [%s] from host [%s]. Error: %#v",
				detachRequest.Name,
				detachRequest.Host,
				err)
			response = k8sresources.FlexVolumeResponse{Status: "Failure", Message: msg, Device: ""}
		} else {
			response = k8sresources.FlexVolumeResponse{
				Status: "Success",
			}
		}
	}

	c.logger.Debug("", logs.Args{{"response", response}})
	return response
}

//MountDevice mounts a device in a given location
func (c *Controller) MountDevice(mountDeviceRequest k8sresources.FlexVolumeMountDeviceRequest) k8sresources.FlexVolumeResponse {
	defer c.logger.Trace(logs.DEBUG)()
	var response k8sresources.FlexVolumeResponse
	c.logger.Debug("", logs.Args{{"request", mountDeviceRequest}})

	response = k8sresources.FlexVolumeResponse{
		Status: "Not supported",
	}

	c.logger.Debug("", logs.Args{{"response", response}})
	return response
}

//UnmountDevice checks if volume is unmounted
func (c *Controller) UnmountDevice(unmountDeviceRequest k8sresources.FlexVolumeUnmountDeviceRequest) k8sresources.FlexVolumeResponse {
	defer c.logger.Trace(logs.DEBUG)()
	var response k8sresources.FlexVolumeResponse
	c.logger.Debug("", logs.Args{{"request", unmountDeviceRequest}})

	response = k8sresources.FlexVolumeResponse{
		Status: "Not supported",
	}

	c.logger.Debug("", logs.Args{{"response", response}})
	return response
}

//Mount method allows to mount the volume/fileset to a given location for a pod
func (c *Controller) Mount(mountRequest k8sresources.FlexVolumeMountRequest) k8sresources.FlexVolumeResponse {
	defer c.logger.Trace(logs.DEBUG)()
	var response k8sresources.FlexVolumeResponse
	c.logger.Debug("", logs.Args{{"request", mountRequest}})

	mountedPath, err := c.doMount(mountRequest)
	if err != nil {
		response = k8sresources.FlexVolumeResponse{
			Status:  "Failure",
			Message: err.Error(),
		}
	} else {
		err = c.doAfterMount(mountRequest, mountedPath)
		if err != nil {
			response = k8sresources.FlexVolumeResponse{
				Status:  "Failure",
				Message: err.Error(),
			}
		} else {
			response = k8sresources.FlexVolumeResponse{
				Status: "Success",
			}
		}
	}

	c.logger.Debug("", logs.Args{{"response", response}})
	return response
}

//Unmount methods unmounts the volume from the pod
func (c *Controller) Unmount(unmountRequest k8sresources.FlexVolumeUnmountRequest) k8sresources.FlexVolumeResponse {
	defer c.logger.Trace(logs.DEBUG)()
	// locking for concurrent rescans and reduce rescans if no need
	c.logger.Debug("Ask for unmountFlock for mountpath", logs.Args{{"mountpath", unmountRequest.MountPath}})
	for {
		err := c.unmountFlock.TryLock()
		if err == nil {
			break
		}
		c.logger.Debug("unmountFlock.TryLock failed", logs.Args{{"error", err}})
		time.Sleep(time.Duration(500*time.Millisecond))
	}
	c.logger.Debug("Got unmountFlock for mountpath", logs.Args{{"mountpath", unmountRequest.MountPath}})
	defer c.unmountFlock.Unlock()
	defer c.logger.Debug("Released unmountFlock for mountpath", logs.Args{{"mountpath", unmountRequest.MountPath}})

	var response k8sresources.FlexVolumeResponse
	c.logger.Debug("", logs.Args{{"request", unmountRequest}})
	var err error

	// Validate that the mountpoint is a symlink as ubiquity expect it to be
	realMountPoint, err := c.exec.EvalSymlinks(unmountRequest.MountPath)
	if err != nil {
		msg := fmt.Sprintf("Cannot execute umount because the mountPath [%s] is not a symlink as expected. Error: %#v", unmountRequest.MountPath, err)
		c.logger.Error(msg)
		return k8sresources.FlexVolumeResponse{Status: "Failure", Message: msg, Device: ""}
	}

	ubiquityMountPrefix := fmt.Sprintf(resources.PathToMountUbiquityBlockDevices, "")
	if strings.HasPrefix(realMountPoint, ubiquityMountPrefix) {
		// SCBE backend flow
		err = c.doUnmountScbe(unmountRequest, realMountPoint)
	} else {
		// SSC backend flow
		err = c.doUnmountSsc(unmountRequest, realMountPoint)
	}

	if err != nil {
		response = k8sresources.FlexVolumeResponse{
			Status:  "Failure",
			Message: err.Error(),
		}
	} else {
		err = c.doLegacyDetach(unmountRequest)
		if err != nil {
			response = k8sresources.FlexVolumeResponse{
				Status:  "Failure",
				Message: err.Error(),
			}
		} else {
			response = k8sresources.FlexVolumeResponse{
				Status: "Success",
			}
		}
	}

	c.logger.Debug("", logs.Args{{"response", response}})
	return response
}

func (c *Controller) doLegacyDetach(unmountRequest k8sresources.FlexVolumeUnmountRequest) error	{
	defer c.logger.Trace(logs.DEBUG)()
	var err error

	pvName := path.Base(unmountRequest.MountPath)
	detachRequest := k8sresources.FlexVolumeDetachRequest{Name: pvName}
	err = c.doDetach(detachRequest, false)
	if err != nil {
		return c.logger.ErrorRet(err, "failed")
	} else {
		err = c.doAfterDetach(detachRequest)
		if err != nil {
			return c.logger.ErrorRet(err, "failed")
		}
	}

	return nil
}

func (c *Controller) getMounterForBackend(backend string) (resources.Mounter, error) {
	defer c.logger.Trace(logs.DEBUG)()
	mounterInst, ok := c.mounterPerBackend[backend]
	if ok {
		return mounterInst, nil
	} else if backend == resources.SpectrumScale {
		c.mounterPerBackend[backend] = mounter.NewSpectrumScaleMounter(c.legacyLogger)
	} else if backend == resources.SoftlayerNFS || backend == resources.SpectrumScaleNFS {
		c.mounterPerBackend[backend] = mounter.NewNfsMounter(c.legacyLogger)
	} else if backend == resources.SCBE {
		c.mounterPerBackend[backend] = mounter.NewScbeMounter(c.config.ScbeRemoteConfig)
	} else {
		err := fmt.Errorf("Mounter not found for backend: %s", backend)
		return nil, c.logger.ErrorRet(err, "failed")
	}
	return c.mounterPerBackend[backend], nil
}

func (c *Controller) doMount(mountRequest k8sresources.FlexVolumeMountRequest) (string, error) {
	defer c.logger.Trace(logs.DEBUG)()

	name := mountRequest.MountDevice
	getVolumeConfigRequest := resources.GetVolumeConfigRequest{Name: name}
	volumeConfig, err := c.Client.GetVolumeConfig(getVolumeConfigRequest)
	if err != nil {
		return "", c.logger.ErrorRet(err, "Client.GetVolumeConfig failed")
	}

	getVolumeRequest := resources.GetVolumeRequest{Name: name}
	volume, err := c.Client.GetVolume(getVolumeRequest)
	mounter, err := c.getMounterForBackend(volume.Backend)
	if err != nil {
		err = fmt.Errorf("Error determining mounter for volume: %s", err.Error())
		return "", c.logger.ErrorRet(err, "getMounterForBackend failed")
	}

	wwn, ok := mountRequest.Opts["Wwn"]
	if !ok {
		err = fmt.Errorf("mountRequest.Opts[Wwn] not found")
		return "", c.logger.ErrorRet(err, "failed")
	}

	volumeMountpoint := fmt.Sprintf(resources.PathToMountUbiquityBlockDevices, wwn)
	ubMountRequest := resources.MountRequest{Mountpoint: volumeMountpoint, VolumeConfig: volumeConfig}
	mountpoint, err := mounter.Mount(ubMountRequest)
	if err != nil {
		return "", c.logger.ErrorRet(err, "mounter.Mount failed")
	}

	return mountpoint, nil
}

func (c *Controller) doAfterMount(mountRequest k8sresources.FlexVolumeMountRequest, mountedPath string) error {
	defer c.logger.Trace(logs.DEBUG)()
	var lnPath string
	var err error

	if mountRequest.Version == k8sresources.KubernetesVersion_1_5 {
		//For k8s 1.5, by the time we do the attach/mount, the mountDir (MountPath) is not created trying to do mount and ln will fail because the dir is not found, so we need to create the directory before continuing
		dir := filepath.Dir(mountRequest.MountPath)
		lnPath = mountRequest.MountPath
		k8sRequiredMountPoint := path.Join(mountRequest.MountPath, mountRequest.MountDevice)
		if _, err = os.Stat(k8sRequiredMountPoint); err != nil {
			if os.IsNotExist(err) {
				c.logger.Debug("creating volume directory", logs.Args{{"dir", dir}})
				err = os.MkdirAll(dir, 0777)
				if err != nil && !os.IsExist(err) {
					err = fmt.Errorf("Failed creating volume directory %#v", err)
					return c.logger.ErrorRet(err, "failed")
				}
			}
		}
	} else {
		// For k8s 1.6 and later kubelet creates a folder as the MountPath, including the volume name, whenwe try to create the symlink this will fail because the same name exists. This is why we need to remove it before continuing.
		ubiquityMountPrefix := fmt.Sprintf(resources.PathToMountUbiquityBlockDevices, "")
		if strings.HasPrefix(mountedPath, ubiquityMountPrefix) {
			lnPath = mountRequest.MountPath
		} else {
			lnPath, _ = path.Split(mountRequest.MountPath)
		}
		c.logger.Debug("removing folder", logs.Args{{"folder", mountRequest.MountPath}})
		err = os.Remove(mountRequest.MountPath)
		if err != nil && !os.IsExist(err) {
			err = fmt.Errorf("Failed removing existing volume directory %#v", err)
			return c.logger.ErrorRet(err, "failed")
		}
	}

	symLinkCommand := "/bin/ln"
	args := []string{"-s", mountedPath, lnPath}
	c.logger.Debug(fmt.Sprintf("creating slink from %s -> %s", mountedPath, lnPath))
	var stderr bytes.Buffer
	cmd := exec.Command(symLinkCommand, args...)
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("Controller: mount failed to symlink %#v", stderr.String())
		c.logger.ErrorRet(err, "failed")
	}

	c.logger.Debug("Volume mounted successfully", logs.Args{{"mountedPath", mountedPath}})
	return nil
}

func (c *Controller) doUnmountScbe(unmountRequest k8sresources.FlexVolumeUnmountRequest, realMountPoint string) error {
	defer c.logger.Trace(logs.DEBUG)()

	pvName := path.Base(unmountRequest.MountPath)
	getVolumeRequest := resources.GetVolumeRequest{Name: pvName}

	volume, err := c.Client.GetVolume(getVolumeRequest)
	mounter, err := c.getMounterForBackend(volume.Backend)
	if err != nil {
		err = fmt.Errorf("Error determining mounter for volume: %s", err.Error())
		return c.logger.ErrorRet(err, "failed")
	}

	getVolumeConfigRequest := resources.GetVolumeConfigRequest{Name: pvName}
	volumeConfig, err := c.Client.GetVolumeConfig(getVolumeConfigRequest)
	if err != nil {
		err = fmt.Errorf("Error unmount for volume: %s", err.Error())
		return c.logger.ErrorRet(err, "Client.GetVolumeConfig failed")
	}

	ubUnmountRequest := resources.UnmountRequest{VolumeConfig: volumeConfig}
	err = mounter.Unmount(ubUnmountRequest)
	if err != nil {
		return c.logger.ErrorRet(err, "mounter.Unmount failed")
	}

	c.logger.Debug(fmt.Sprintf("Removing the slink [%s] to the real mountpoint [%s]", unmountRequest.MountPath, realMountPoint))
	err = c.exec.Remove(unmountRequest.MountPath)
	if err != nil {
		err = fmt.Errorf("fail to remove slink %s. Error %#v", unmountRequest.MountPath, err)
		return c.logger.ErrorRet(err, "exec.Remove failed")
	}

	return nil
}

func (c *Controller) doAfterDetach(detachRequest k8sresources.FlexVolumeDetachRequest) error {
    defer c.logger.Trace(logs.DEBUG)()

    getVolumeRequest := resources.GetVolumeRequest{Name: detachRequest.Name}
    volume, err := c.Client.GetVolume(getVolumeRequest)
    mounter, err := c.getMounterForBackend(volume.Backend)
    if err != nil {
        err = fmt.Errorf("Error determining mounter for volume: %s", err.Error())
        return c.logger.ErrorRet(err, "failed")
    }

    getVolumeConfigRequest := resources.GetVolumeConfigRequest{Name: detachRequest.Name}
    volumeConfig, err := c.Client.GetVolumeConfig(getVolumeConfigRequest)
    if err != nil {
        err = fmt.Errorf("Error for volume: %s", err.Error())
        return c.logger.ErrorRet(err, "Client.GetVolumeConfig failed")
    }

    afterDetachRequest := resources.AfterDetachRequest{VolumeConfig: volumeConfig}
    if err := mounter.ActionAfterDetach(afterDetachRequest); err != nil {
        err = fmt.Errorf("Error execute action after detaching the volume : %#v", err)
        return c.logger.ErrorRet(err, "mounter.ActionAfterDetach failed")
    }

    return nil
}

func (c *Controller) doUnmountSsc(unmountRequest k8sresources.FlexVolumeUnmountRequest, realMountPoint string) error {
    defer c.logger.Trace(logs.DEBUG)()

    listVolumeRequest := resources.ListVolumesRequest{}
    volumes, err := c.Client.ListVolumes(listVolumeRequest)
    if err != nil {
        err = fmt.Errorf("Error getting the volume list from ubiquity server %#v", err)
        return c.logger.ErrorRet(err, "failed")
    }

    volume, err := getVolumeForMountpoint(unmountRequest.MountPath, volumes)
    if err != nil {
        err = fmt.Errorf(
            "Error finding the volume with mountpoint [%s] from the list of ubiquity volumes %#v. Error is : %#v",
            unmountRequest.MountPath,
            volumes,
            err)
        return c.logger.ErrorRet(err, "failed")
    }

    detachRequest := resources.DetachRequest{Name: volume.Name}
    err = c.Client.Detach(detachRequest)
    if err != nil && err.Error() != "fileset not linked" {
        err = fmt.Errorf(
            "Failed to unmount volume [%s] on mountpoint [%s]. Error: %#v",
            volume.Name,
            unmountRequest.MountPath,
            err)
        return c.logger.ErrorRet(err, "failed")
    }

    return nil
}

func (c *Controller) doActivate(activateRequest resources.ActivateRequest) error {
	defer c.logger.Trace(logs.DEBUG)()

	err := c.Client.Activate(activateRequest)
	if err != nil {
		return c.logger.ErrorRet(err, "Client.Activate failed")
	}

	return nil
}

func (c *Controller) doAttach(attachRequest k8sresources.FlexVolumeAttachRequest) error {
	defer c.logger.Trace(logs.DEBUG)()

	ubAttachRequest := resources.AttachRequest{Name: attachRequest.Name, Host: getHost(attachRequest.Host)}
	_, err := c.Client.Attach(ubAttachRequest)
	if err != nil {
		return c.logger.ErrorRet(err, "Client.Attach failed")
	}

	return nil
}

func (c *Controller) doDetach(detachRequest k8sresources.FlexVolumeDetachRequest, checkIfAttached bool) error {
	defer c.logger.Trace(logs.DEBUG)()

	if checkIfAttached {
		opts := make(map[string]string)
		opts["volumeName"] = detachRequest.Name
		isAttachedRequest := k8sresources.FlexVolumeIsAttachedRequest{Name: "", Host:detachRequest.Host, Opts: opts}
		isAttached, err := c.doIsAttached(isAttachedRequest)
		if err != nil {
			return c.logger.ErrorRet(err, "failed")
		}
		if !isAttached {
			return nil
		}
	}
	host := detachRequest.Host
	if host == "" {
		// only when triggered during unmount
		var err error
		host, err = c.getHostAttached(detachRequest.Name)
		if err != nil {
			return c.logger.ErrorRet(err, "getHostAttached failed")
		}
	}


	ubDetachRequest := resources.DetachRequest{Name: detachRequest.Name, Host: host}
	err := c.Client.Detach(ubDetachRequest)
	if err != nil {
		return c.logger.ErrorRet(err, "failed")
	}

	return nil
}

func (c *Controller) doIsAttached(isAttachedRequest k8sresources.FlexVolumeIsAttachedRequest) (bool, error) {
	defer c.logger.Trace(logs.DEBUG)()

	volName, ok := isAttachedRequest.Opts["volumeName"]
	if !ok {
		err := fmt.Errorf("volumeName not found in isAttachedRequest")
		return false, c.logger.ErrorRet(err, "failed")
	}

	attachTo, err := c.getHostAttached(volName)
	if err != nil {
		return false, c.logger.ErrorRet(err, "getHostAttached failed")
	}

	isAttached := isAttachedRequest.Host == attachTo
	c.logger.Debug("", logs.Args{{"host", isAttachedRequest.Host}, {"attachTo", attachTo}, {"isAttached", isAttached}})
	return isAttached, nil
}

func (c *Controller) getHostAttached(volName string) (string, error) {
	defer c.logger.Trace(logs.DEBUG)()

	getVolumeConfigRequest := resources.GetVolumeConfigRequest{Name: volName}
	volumeConfig, err := c.Client.GetVolumeConfig(getVolumeConfigRequest)
	if err != nil {
		return "", c.logger.ErrorRet(err, "Client.GetVolumeConfig failed")
	}

	attachTo, ok := volumeConfig[resources.ScbeKeyVolAttachToHost].(string)
	if !ok {
		return "", c.logger.ErrorRet(err, "GetVolumeConfig missing info", logs.Args{{"arg", resources.ScbeKeyVolAttachToHost}})
	}
	c.logger.Debug("", logs.Args{{"volumeConfig", volumeConfig}, {"attachTo", attachTo}})

	return attachTo, nil
}

func getVolumeForMountpoint(mountpoint string, volumes []resources.Volume) (resources.Volume, error) {
	for _, volume := range volumes {
		if volume.Mountpoint == mountpoint {
			return volume, nil
		}
	}
	return resources.Volume{}, fmt.Errorf("Volume not found")
}

func getHost(hostRequest string) string {
    if hostRequest != "" {
        return hostRequest
    }
    // Only in k8s 1.5 this os.Hostname will happened,
    // because in k8s 1.5 the flex CLI doesn't get the host to attach with. TODO consider to refactor to remove support for 1.5
    hostname, err := os.Hostname()
    if err != nil {
        return ""
    }
    return hostname
}