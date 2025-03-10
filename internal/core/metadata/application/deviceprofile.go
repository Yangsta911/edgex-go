//
// Copyright (C) 2020-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"

	"github.com/edgexfoundry/edgex-go/internal/core/metadata/container"
	"github.com/edgexfoundry/edgex-go/internal/core/metadata/infrastructure/interfaces"
	"github.com/edgexfoundry/edgex-go/internal/pkg/correlation"
	"github.com/edgexfoundry/edgex-go/internal/pkg/utils"

	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
)

// The AddDeviceProfile function accepts the new device profile model from the controller functions
// and invokes addDeviceProfile function in the infrastructure layer
func AddDeviceProfile(d models.DeviceProfile, ctx context.Context, dic *di.Container) (id string, err errors.EdgeX) {
	dbClient := container.DBClientFrom(dic.Get)
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)

	err = deviceProfileUoMValidation(d, dic)
	if err != nil {
		return "", errors.NewCommonEdgeXWrapper(err)
	}

	correlationId := correlation.FromContext(ctx)
	addedDeviceProfile, err := dbClient.AddDeviceProfile(d)
	if err != nil {
		return "", errors.NewCommonEdgeXWrapper(err)
	}

	lc.Debugf(
		"DeviceProfile created on DB successfully. DeviceProfile-id: %s, Correlation-id: %s ",
		addedDeviceProfile.Id,
		correlationId,
	)

	profileDTO := dtos.FromDeviceProfileModelToDTO(addedDeviceProfile)
	go publishSystemEvent(common.DeviceProfileSystemEventType, common.SystemEventActionAdd, common.CoreMetaDataServiceKey, profileDTO, ctx, dic)

	return addedDeviceProfile.Id, nil
}

// The UpdateDeviceProfile function accepts the device profile model from the controller functions
// and invokes updateDeviceProfile function in the infrastructure layer
func UpdateDeviceProfile(d models.DeviceProfile, ctx context.Context, dic *di.Container) (err errors.EdgeX) {
	dbClient := container.DBClientFrom(dic.Get)
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	config := container.ConfigurationFrom(dic.Get)

	err = deviceProfileUoMValidation(d, dic)
	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}

	if config.Writable.MaxResources > 0 {
		if err = checkResourceCapacityByUpdateProfile(d, dic); err != nil {
			return errors.NewCommonEdgeXWrapper(err)
		}
	}

	err = dbClient.UpdateDeviceProfile(d)
	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}

	lc.Debugf(
		"DeviceProfile updated on DB successfully. Correlation-id: %s ",
		correlation.FromContext(ctx),
	)

	profile, err := dbClient.DeviceProfileByName(d.Name)
	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}

	profileDTO := dtos.FromDeviceProfileModelToDTO(profile)
	go publishUpdateDeviceProfileSystemEvent(profileDTO, ctx, dic)

	return nil
}

func isProfileInUse(profileName string, dic *di.Container) (bool, errors.EdgeX) {
	dbClient := container.DBClientFrom(dic.Get)
	count, err := dbClient.DeviceCountByProfileName(profileName)
	if err != nil {
		return false, errors.NewCommonEdgeXWrapper(err)
	}
	return count > 0, nil
}

// DeviceProfileByName query the device profile by name
func DeviceProfileByName(name string, ctx context.Context, dic *di.Container) (deviceProfile dtos.DeviceProfile, err errors.EdgeX) {
	if name == "" {
		return deviceProfile, errors.NewCommonEdgeX(errors.KindContractInvalid, "name is empty", nil)
	}
	dbClient := container.DBClientFrom(dic.Get)
	dp, err := dbClient.DeviceProfileByName(name)
	if err != nil {
		return deviceProfile, errors.NewCommonEdgeXWrapper(err)
	}
	deviceProfile = dtos.FromDeviceProfileModelToDTO(dp)
	return deviceProfile, nil
}

// DeleteDeviceProfileByName delete the device profile by name
func DeleteDeviceProfileByName(name string, ctx context.Context, dic *di.Container) errors.EdgeX {
	strictProfileDeletes := container.ConfigurationFrom(dic.Get).Writable.ProfileChange.StrictDeviceProfileDeletes
	if strictProfileDeletes {
		return errors.NewCommonEdgeX(errors.KindServiceLocked, "profile deletion is not allowed when StrictDeviceProfileDeletes config is enabled", nil)
	}
	if name == "" {
		return errors.NewCommonEdgeX(errors.KindContractInvalid, "name is empty", nil)
	}
	dbClient := container.DBClientFrom(dic.Get)
	profile, err := dbClient.DeviceProfileByName(name)
	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}
	// Check the associated Device and ProvisionWatcher existence
	devices, edgeXErr := dbClient.DevicesByProfileName(0, 1, name)
	if edgeXErr != nil {
		return errors.NewCommonEdgeXWrapper(edgeXErr)
	}
	if len(devices) > 0 {
		return errors.NewCommonEdgeX(errors.KindStatusConflict, "fail to delete the device profile when associated device exists", nil)
	}
	provisionWatchers, edgeXErr := dbClient.ProvisionWatchersByProfileName(0, 1, name)
	if edgeXErr != nil {
		return errors.NewCommonEdgeXWrapper(edgeXErr)
	}
	if len(provisionWatchers) > 0 {
		return errors.NewCommonEdgeX(errors.KindStatusConflict, "fail to delete the device profile when associated provisionWatcher exists", nil)
	}

	err = dbClient.DeleteDeviceProfileByName(name)
	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}

	profileDTO := dtos.FromDeviceProfileModelToDTO(profile)
	go publishSystemEvent(common.DeviceProfileSystemEventType, common.SystemEventActionDelete, common.CoreMetaDataServiceKey, profileDTO, ctx, dic)

	return nil
}

// AllDeviceProfiles query the device profiles with offset, and limit
func AllDeviceProfiles(offset int, limit int, labels []string, dic *di.Container) (deviceProfiles []dtos.DeviceProfile, totalCount uint32, err errors.EdgeX) {
	dbClient := container.DBClientFrom(dic.Get)

	totalCount, err = dbClient.DeviceProfileCountByLabels(labels)
	if err != nil {
		return deviceProfiles, totalCount, errors.NewCommonEdgeXWrapper(err)
	}
	cont, err := utils.CheckCountRange(totalCount, offset, limit)
	if !cont {
		return []dtos.DeviceProfile{}, totalCount, err
	}

	dps, err := dbClient.AllDeviceProfiles(offset, limit, labels)
	if err != nil {
		return deviceProfiles, totalCount, errors.NewCommonEdgeXWrapper(err)
	}
	deviceProfiles = make([]dtos.DeviceProfile, len(dps))
	for i, dp := range dps {
		deviceProfiles[i] = dtos.FromDeviceProfileModelToDTO(dp)
	}
	return deviceProfiles, totalCount, nil
}

// DeviceProfilesByModel query the device profiles with offset, limit and model
func DeviceProfilesByModel(offset int, limit int, model string, dic *di.Container) (deviceProfiles []dtos.DeviceProfile, totalCount uint32, err errors.EdgeX) {
	if model == "" {
		return deviceProfiles, totalCount, errors.NewCommonEdgeX(errors.KindContractInvalid, "model is empty", nil)
	}

	dbClient := container.DBClientFrom(dic.Get)
	totalCount, err = dbClient.DeviceProfileCountByModel(model)
	if err != nil {
		return deviceProfiles, totalCount, errors.NewCommonEdgeXWrapper(err)
	}
	cont, err := utils.CheckCountRange(totalCount, offset, limit)
	if !cont {
		return []dtos.DeviceProfile{}, totalCount, err
	}

	dps, err := dbClient.DeviceProfilesByModel(offset, limit, model)
	if err != nil {
		return deviceProfiles, totalCount, errors.NewCommonEdgeXWrapper(err)
	}
	deviceProfiles = make([]dtos.DeviceProfile, len(dps))
	for i, dp := range dps {
		deviceProfiles[i] = dtos.FromDeviceProfileModelToDTO(dp)
	}
	return deviceProfiles, totalCount, nil
}

// DeviceProfilesByManufacturer query the device profiles with offset, limit and manufacturer
func DeviceProfilesByManufacturer(offset int, limit int, manufacturer string, dic *di.Container) (deviceProfiles []dtos.DeviceProfile, totalCount uint32, err errors.EdgeX) {
	if manufacturer == "" {
		return deviceProfiles, totalCount, errors.NewCommonEdgeX(errors.KindContractInvalid, "manufacturer is empty", nil)
	}

	dbClient := container.DBClientFrom(dic.Get)
	totalCount, err = dbClient.DeviceProfileCountByManufacturer(manufacturer)
	if err != nil {
		return deviceProfiles, totalCount, errors.NewCommonEdgeXWrapper(err)
	}
	cont, err := utils.CheckCountRange(totalCount, offset, limit)
	if !cont {
		return []dtos.DeviceProfile{}, totalCount, err
	}

	dps, err := dbClient.DeviceProfilesByManufacturer(offset, limit, manufacturer)
	if err != nil {
		return deviceProfiles, totalCount, errors.NewCommonEdgeXWrapper(err)
	}
	deviceProfiles = make([]dtos.DeviceProfile, len(dps))
	for i, dp := range dps {
		deviceProfiles[i] = dtos.FromDeviceProfileModelToDTO(dp)
	}
	return deviceProfiles, totalCount, nil
}

// DeviceProfilesByManufacturerAndModel query the device profiles with offset, limit, manufacturer and model
func DeviceProfilesByManufacturerAndModel(offset int, limit int, manufacturer string, model string, dic *di.Container) (deviceProfiles []dtos.DeviceProfile, totalCount uint32, err errors.EdgeX) {
	if manufacturer == "" {
		return deviceProfiles, totalCount, errors.NewCommonEdgeX(errors.KindContractInvalid, "manufacturer is empty", nil)
	}
	if model == "" {
		return deviceProfiles, totalCount, errors.NewCommonEdgeX(errors.KindContractInvalid, "model is empty", nil)
	}
	dbClient := container.DBClientFrom(dic.Get)
	totalCount, err = dbClient.DeviceProfileCountByManufacturerAndModel(manufacturer, model)
	if err != nil {
		return deviceProfiles, totalCount, errors.NewCommonEdgeXWrapper(err)
	}
	cont, err := utils.CheckCountRange(totalCount, offset, limit)
	if !cont {
		return []dtos.DeviceProfile{}, totalCount, err
	}

	dps, err := dbClient.DeviceProfilesByManufacturerAndModel(offset, limit, manufacturer, model)
	if err != nil {
		return deviceProfiles, totalCount, errors.NewCommonEdgeXWrapper(err)
	}
	deviceProfiles = make([]dtos.DeviceProfile, len(dps))
	for i, dp := range dps {
		deviceProfiles[i] = dtos.FromDeviceProfileModelToDTO(dp)
	}
	return deviceProfiles, totalCount, nil
}

func PatchDeviceProfileBasicInfo(ctx context.Context, dto dtos.UpdateDeviceProfileBasicInfo, dic *di.Container) errors.EdgeX {
	dbClient := container.DBClientFrom(dic.Get)
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)

	deviceProfile, err := deviceProfileByDTO(dbClient, dto)
	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}

	requests.ReplaceDeviceProfileModelBasicInfoFieldsWithDTO(&deviceProfile, dto)
	err = dbClient.UpdateDeviceProfile(deviceProfile)
	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}

	lc.Debugf(
		"DeviceProfile basic info patched on DB successfully. Correlation-ID: %s ",
		correlation.FromContext(ctx),
	)

	profileDTO := dtos.FromDeviceProfileModelToDTO(deviceProfile)
	go publishUpdateDeviceProfileSystemEvent(profileDTO, ctx, dic)

	return nil
}

// AllDeviceProfileBasicInfos query the device profile basic infos with offset, and limit
func AllDeviceProfileBasicInfos(offset int, limit int, labels []string, dic *di.Container) (deviceProfileBasicInfos []dtos.DeviceProfileBasicInfo, totalCount uint32, err errors.EdgeX) {
	dbClient := container.DBClientFrom(dic.Get)

	totalCount, err = dbClient.DeviceProfileCountByLabels(labels)
	if err != nil {
		return deviceProfileBasicInfos, totalCount, errors.NewCommonEdgeXWrapper(err)
	}
	cont, err := utils.CheckCountRange(totalCount, offset, limit)
	if !cont {
		return deviceProfileBasicInfos, totalCount, err
	}

	dps, err := dbClient.AllDeviceProfiles(offset, limit, labels)
	if err != nil {
		return deviceProfileBasicInfos, totalCount, errors.NewCommonEdgeXWrapper(err)
	}
	deviceProfileBasicInfos = make([]dtos.DeviceProfileBasicInfo, len(dps))
	for i, dp := range dps {
		deviceProfileBasicInfos[i] = dtos.FromDeviceProfileModelToBasicInfoDTO(dp)
	}
	return deviceProfileBasicInfos, totalCount, nil
}

func deviceProfileByDTO(dbClient interfaces.DBClient, dto dtos.UpdateDeviceProfileBasicInfo) (deviceProfile models.DeviceProfile, err errors.EdgeX) {
	// The ID or Name is required by DTO and the DTO also accepts empty string ID if the Name is provided
	if dto.Id != nil && *dto.Id != "" {
		deviceProfile, err = dbClient.DeviceProfileById(*dto.Id)
		if err != nil {
			return deviceProfile, errors.NewCommonEdgeXWrapper(err)
		}
	} else {
		deviceProfile, err = dbClient.DeviceProfileByName(*dto.Name)
		if err != nil {
			return deviceProfile, errors.NewCommonEdgeXWrapper(err)
		}
	}
	if dto.Name != nil && *dto.Name != deviceProfile.Name {
		return deviceProfile, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("device profile name '%s' not match the exsting '%s' ", *dto.Name, deviceProfile.Name), nil)
	}
	return deviceProfile, nil
}

func deviceProfileUoMValidation(p models.DeviceProfile, dic *di.Container) errors.EdgeX {
	if container.ConfigurationFrom(dic.Get).Writable.UoM.Validation {
		uom := container.UnitsOfMeasureFrom(dic.Get)
		for _, dr := range p.DeviceResources {
			if ok := uom.Validate(dr.Properties.Units); !ok {
				return errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("DeviceResource %s units %s is invalid", dr.Name, dr.Properties.Units), nil)
			}
		}
	}

	return nil
}
