/*******************************************************************************
 * Copyright 2018 Dell Technologies Inc.
 * Copyright 2023 Intel Corporation
 * Copyright 2025 IOTech Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *
 *******************************************************************************/

package config

import (
	bootstrapConfig "github.com/edgexfoundry/go-mod-bootstrap/v4/config"
)

type ConfigurationStruct struct {
	Writable   WritableInfo
	Clients    bootstrapConfig.ClientsCollection
	Database   bootstrapConfig.Database
	Registry   bootstrapConfig.RegistryInfo
	Service    bootstrapConfig.ServiceInfo
	MessageBus bootstrapConfig.MessageBusInfo
	Smtp       SmtpInfo
	Retention  NotificationRetention
}

type WritableInfo struct {
	LogLevel string
	// ResendLimit is the default retry limit for attempts to send notifications.
	ResendLimit int
	// ResendInterval is the default interval of resending the notification. The format of this field is to be an unsigned integer followed by a unit which may be "ns", "us" (or "µs"), "ms", "s", "m", "h" representing nanoseconds, microseconds, milliseconds, seconds, minutes or hours. Eg, "100ms", "24h"
	ResendInterval  string
	InsecureSecrets bootstrapConfig.InsecureSecrets
	Telemetry       bootstrapConfig.TelemetryInfo
}

type SmtpInfo struct {
	Host                 string
	Port                 int
	Sender               string
	EnableSelfSignedCert bool
	Subject              string
	// SecretName is used to specify the secret path to store the credential(username and password) for connecting the SMTP server
	// User need to store the credential via the /secret API before sending the email notification
	SecretName string
	// AuthMode is the SMTP authentication mechanism. Currently, 'usernamepassword' is the only AuthMode supported by this service, and the secret keys are 'username' and 'password'.
	AuthMode string
}

type NotificationRetention struct {
	Enabled  bool
	Interval string
	MaxCap   uint32
	MinCap   uint32
}

// UpdateFromRaw converts configuration received from the registry to a service-specific configuration struct which is
// then used to overwrite the service's existing configuration struct.
func (c *ConfigurationStruct) UpdateFromRaw(rawConfig interface{}) bool {
	configuration, ok := rawConfig.(*ConfigurationStruct)
	if ok {
		*c = *configuration
	}
	return ok
}

// EmptyWritablePtr returns a pointer to a service-specific empty WritableInfo struct.  It is used by the bootstrap to
// provide the appropriate structure to registry.Client's WatchForChanges().
func (c *ConfigurationStruct) EmptyWritablePtr() interface{} {
	return &WritableInfo{}
}

// GetWritablePtr returns pointer to the writable section
func (c *ConfigurationStruct) GetWritablePtr() any {
	return &c.Writable
}

// UpdateWritableFromRaw converts configuration received from the registry to a service-specific WritableInfo struct
// which is then used to overwrite the service's existing configuration's WritableInfo struct.
func (c *ConfigurationStruct) UpdateWritableFromRaw(rawWritable interface{}) bool {
	writable, ok := rawWritable.(*WritableInfo)
	if ok {
		c.Writable = *writable
	}
	return ok
}

// GetBootstrap returns the configuration elements required by the bootstrap.  Currently, a copy of the configuration
// data is returned.  This is intended to be temporary -- since ConfigurationStruct drives the configuration.yaml's
// structure -- until we can make backwards-breaking configuration.yaml changes (which would consolidate these fields
// into an bootstrapConfig.BootstrapConfiguration struct contained within ConfigurationStruct).
func (c *ConfigurationStruct) GetBootstrap() bootstrapConfig.BootstrapConfiguration {
	// temporary until we can make backwards-breaking configuration.yaml change
	return bootstrapConfig.BootstrapConfiguration{
		Clients:    &c.Clients,
		Service:    &c.Service,
		Registry:   &c.Registry,
		MessageBus: &c.MessageBus,
		Database:   &c.Database,
	}
}

// GetLogLevel returns the current ConfigurationStruct's log level.
func (c *ConfigurationStruct) GetLogLevel() string {
	return c.Writable.LogLevel
}

// GetRegistryInfo returns the RegistryInfo from the ConfigurationStruct.
func (c *ConfigurationStruct) GetRegistryInfo() bootstrapConfig.RegistryInfo {
	return c.Registry
}

// GetDatabaseInfo returns a database information.
func (c *ConfigurationStruct) GetDatabaseInfo() bootstrapConfig.Database {
	return c.Database
}

// GetInsecureSecrets returns the service's InsecureSecrets.
func (c *ConfigurationStruct) GetInsecureSecrets() bootstrapConfig.InsecureSecrets {
	return c.Writable.InsecureSecrets
}

// GetTelemetryInfo returns the service's Telemetry settings.
func (c *ConfigurationStruct) GetTelemetryInfo() *bootstrapConfig.TelemetryInfo {
	return &c.Writable.Telemetry
}
