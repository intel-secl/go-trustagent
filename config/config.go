/*
 * Copyright (C) 2020 Intel Corporation
 * SPDX-License-Identifier: BSD-3-Clause
 */
package config

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"intel/isecl/go-trust-agent/v4/constants"
	"intel/isecl/lib/common/v4/setup"

	commLog "github.com/intel-secl/intel-secl/v4/pkg/lib/common/log"
	"github.com/intel-secl/intel-secl/v4/pkg/lib/common/log/message"
	commLogInt "github.com/intel-secl/intel-secl/v4/pkg/lib/common/log/setup"
	"github.com/intel-secl/intel-secl/v4/pkg/lib/common/validation"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type WebService struct {
	Port              int           // TRUSTAGENT_PORT
	ReadTimeout       time.Duration // TA_SERVER_READ_TIMEOUT
	ReadHeaderTimeout time.Duration // TA_SERVER_READ_HEADER_TIMEOUT
	WriteTimeout      time.Duration // TA_SERVER_WRITE_TIMEOUT
	IdleTimeout       time.Duration // TA_SERVER_IDLE_TIMEOUT
	MaxHeaderBytes    int           // TA_SERVER_MAX_HEADER_BYTES
}

type NatsService struct {
	Servers []string
	HostID  string
}

type TrustAgentConfiguration struct {
	configFile string
	Mode       string
	Logging    struct {
		LogLevel          string // TRUSTAGENT_LOG_LEVEL
		LogEnableStdout   bool   // TA_ENABLE_CONSOLE_LOG
		LogEntryMaxLength int    // LOG_ENTRY_MAXLENGTH (NEEDS TO BE IN LLD)
	}
	WebService WebService
	HVS        struct {
		Url string // HVS_URL
	}
	Tpm struct {
		TagSecretKey string
	}
	AAS struct {
		BaseURL string // AAS_API_URL
	}
	CMS struct {
		BaseURL       string // CMS_BASE_URL
		TLSCertDigest string // CMS_TLS_CERT_SHA384
	}
	TLS struct {
		CertSAN string // SAN_LIST
		CertCN  string // TA_TLS_CERT_CN
	}
	Nats     NatsService
	ApiToken string
}

var mu sync.Mutex
var log = commLog.GetDefaultLogger()
var secLog = commLog.GetSecurityLogger()

func NewConfigFromYaml(pathToYaml string) (*TrustAgentConfiguration, error) {

	var c TrustAgentConfiguration
	file, err := os.Open(pathToYaml)
	if err == nil {
		defer func() {
			derr := file.Close()
			if derr != nil {
				log.WithError(derr).Warn("Error closing file")
			}
		}()
		err = yaml.NewDecoder(file).Decode(&c)
		if err != nil {
			return nil, err
		}
	} else {
		// file doesnt exist, create a new blank one
		c.Logging.LogLevel = logrus.InfoLevel.String()
	}

	c.configFile = pathToYaml
	return &c, nil
}

var ErrNoConfigFile = errors.New("no config file")

func (cfg *TrustAgentConfiguration) Save() error {

	if cfg.configFile == "" {
		return ErrNoConfigFile
	}

	file, err := os.OpenFile(cfg.configFile, os.O_RDWR|os.O_TRUNC, 0)
	if err != nil {
		// we have an error
		if os.IsNotExist(err) {
			// error is that the config doesnt yet exist, create it
			file, err = os.OpenFile(cfg.configFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			err = os.Chmod(cfg.configFile, 0660)
			if err != nil {
				return err
			}
		} else {
			// someother I/O related error
			return err
		}
	}

	defer func() {
		derr := file.Close()
		if derr != nil {
			log.WithError(derr).Warn("Error closing file")
		}
	}()

	err = yaml.NewEncoder(file).Encode(cfg)
	if err != nil {
		return err
	}

	secLog.Info(message.ConfigChanged)
	log.Debug("Successfully updated config.yaml")
	return nil
}

// This function will load environment variables into the TrustAgentConfiguration
// structure.  It does not validate the presence of env/config values since that
// is handled 'lazily' by setup tasks.
func (cfg *TrustAgentConfiguration) LoadEnvironmentVariables() error {
	var err error
	var context setup.Context
	var environmentVariable string

	//---------------------------------------------------------------------------------------------
	// HVS_URL
	//---------------------------------------------------------------------------------------------
	environmentVariable, err = context.GetenvString(constants.EnvMtwilsonAPIURL, "Verification Service API URL")
	if environmentVariable != "" && cfg.HVS.Url != environmentVariable {
		cfg.HVS.Url = environmentVariable
	}

	//---------------------------------------------------------------------------------------------
	// AAS_API_URL
	//---------------------------------------------------------------------------------------------
	environmentVariable, err = context.GetenvString(constants.EnvAASBaseURL, "AAS API Base URL")
	if environmentVariable != "" && cfg.AAS.BaseURL != environmentVariable {
		if strings.HasSuffix(environmentVariable, "/") {
			cfg.AAS.BaseURL = environmentVariable
		} else {
			cfg.AAS.BaseURL = environmentVariable + "/"
		}
	}

	//---------------------------------------------------------------------------------------------
	// CMS_BASE_URL
	//---------------------------------------------------------------------------------------------
	environmentVariable, err = context.GetenvString(constants.EnvCMSBaseURL, "CMS Base URL")
	if environmentVariable != "" && cfg.CMS.BaseURL != environmentVariable {
		cfg.CMS.BaseURL = environmentVariable
	}

	//---------------------------------------------------------------------------------------------

	// TRUSTAGENT_LOG_LEVEL
	//---------------------------------------------------------------------------------------------
	ll, err := context.GetenvString(constants.EnvTALogLevel, "Logging Level")
	if err == nil {
		llp, err := logrus.ParseLevel(ll)
		if err == nil {
			cfg.Logging.LogLevel = llp.String()
			fmt.Printf("Log level set %s\n", ll)
		}
	}

	if cfg.Logging.LogLevel == "" {
		fmt.Println(constants.EnvTALogLevel, " not defined, using default log level: Info")
		cfg.Logging.LogLevel = logrus.InfoLevel.String()
	}

	//---------------------------------------------------------------------------------------------
	// CMS_TLS_CERT_SHA384
	//---------------------------------------------------------------------------------------------
	environmentVariable, err = context.GetenvString(constants.EnvCMSTLSCertDigest, "CMS TLS SHA384 Digest")
	if environmentVariable != "" {
		if len(environmentVariable) != 96 {
			return errors.Errorf("config/config:LoadEnvironmentVariables()  Invalid length %s: %d", constants.EnvCMSTLSCertDigest, len(environmentVariable))
		}

		if err = validation.ValidateHexString(environmentVariable); err != nil {
			return errors.Errorf("config/config:LoadEnvironmentVariables()  %s is not a valid hex string: %s", constants.EnvCMSTLSCertDigest, environmentVariable)
		}

		if cfg.CMS.TLSCertDigest != environmentVariable {
			cfg.CMS.TLSCertDigest = environmentVariable
		}
	}

	//---------------------------------------------------------------------------------------------
	// TA_TLS_CERT_CN
	//---------------------------------------------------------------------------------------------
	environmentVariable, err = context.GetenvString(constants.EnvTLSCertCommonName, "Trustagent TLS Certificate Common Name")
	if err == nil && environmentVariable != "" {
		cfg.TLS.CertCN = environmentVariable
	} else if strings.TrimSpace(cfg.TLS.CertCN) == "" {
		fmt.Printf("TA_TLS_CERT_CN not defined, using default value %s\n", constants.DefaultTaTlsCn)
		cfg.TLS.CertCN = constants.DefaultTaTlsCn
	}

	//---------------------------------------------------------------------------------------------
	// SAN_LIST
	//---------------------------------------------------------------------------------------------
	environmentVariable, err = context.GetenvString(constants.EnvCertSanList, "Trustagent TLS Certificate SAN LIST")
	if err == nil && environmentVariable != "" {
		cfg.TLS.CertSAN = environmentVariable
	} else if strings.TrimSpace(cfg.TLS.CertSAN) == "" {
		fmt.Printf("SAN_LIST not defined, using default value %s\n", constants.DefaultTaTlsSan)
		cfg.TLS.CertSAN = constants.DefaultTaTlsSan
	}

	//---------------------------------------------------------------------------------------------
	// TA_SERVICE_MODE
	//---------------------------------------------------------------------------------------------
	environmentVariable, err = context.GetenvString(constants.EnvTAServiceMode, "Trustagent Service Mode")
	if err == nil && environmentVariable != "" &&
		(environmentVariable == constants.CommunicationModeHttp || environmentVariable == constants.CommunicationModeOutbound) {
		cfg.Mode = environmentVariable

		if cfg.Mode == constants.CommunicationModeOutbound {
			//---------------------------------------------------------------------------------------------
			// NAT_SERVERS
			//---------------------------------------------------------------------------------------------
			environmentVariable, err = context.GetenvString(constants.EnvNATServers, "NAT servers")
			if err == nil && environmentVariable != "" {
				cfg.Nats.Servers = strings.Split(environmentVariable, ",")
			} else {
				fmt.Println(constants.EnvNATServers, " not defined")
			}

			//---------------------------------------------------------------------------------------------
			// TA_HOST_ID
			//---------------------------------------------------------------------------------------------
			environmentVariable, err = context.GetenvString(constants.EnvTAHostId, "Trustagent Host Id")
			if err == nil && environmentVariable != "" {
				cfg.Nats.HostID = environmentVariable
			} else {
				fmt.Println(constants.EnvTAHostId, " not defined")
			}
		}
	} else {
		fmt.Printf("TA_SERVICE_MODE not provided, using default value %s\n", constants.CommunicationModeHttp)
		cfg.Mode = constants.CommunicationModeHttp
	}

	return nil
}

func (cfg *TrustAgentConfiguration) LogConfiguration(stdOut bool) {
	log.Trace("config/config:LogConfiguration() Entering")
	defer log.Trace("config/config:LogConfiguration() Leaving")

	// creating the log file if not preset
	var ioWriterDefault io.Writer
	var err error = nil
	defaultLogFile, _ := os.OpenFile(constants.DefaultLogFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	err = os.Chmod(constants.DefaultLogFilePath, 0640)
	if err != nil {
		log.Errorf("config/config:LogConfiguration() error in setting file permission for file : %s", constants.DefaultLogFilePath)
	}

	secLogFile, _ := os.OpenFile(constants.SecurityLogFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	err = os.Chmod(constants.SecurityLogFilePath, 0640)
	if err != nil {
		log.Errorf("config/config:LogConfiguration() error in setting file permission for file : %s", constants.SecurityLogFilePath)
	}

	ioWriterDefault = defaultLogFile
	if stdOut {
		ioWriterDefault = io.MultiWriter(os.Stdout, defaultLogFile)
	}
	ioWriterSecurity := io.MultiWriter(ioWriterDefault, secLogFile)

	if cfg.Logging.LogLevel == "" {
		cfg.Logging.LogLevel = logrus.InfoLevel.String()
	}

	llp, err := logrus.ParseLevel(cfg.Logging.LogLevel)
	if err != nil {
		cfg.Logging.LogLevel = logrus.InfoLevel.String()
		llp, _ = logrus.ParseLevel(cfg.Logging.LogLevel)
	}
	commLogInt.SetLogger(commLog.DefaultLoggerName, llp, &commLog.LogFormatter{MaxLength: cfg.Logging.LogEntryMaxLength}, ioWriterDefault, false)
	commLogInt.SetLogger(commLog.SecurityLoggerName, llp, &commLog.LogFormatter{MaxLength: cfg.Logging.LogEntryMaxLength}, ioWriterSecurity, false)

	secLog.Infof("config/config:LogConfiguration() %s", message.LogInit)
	log.Infof("config/config:LogConfiguration() %s", message.LogInit)
}
