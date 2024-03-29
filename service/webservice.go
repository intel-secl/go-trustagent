/*
 * Copyright (C) 2020 Intel Corporation
 * SPDX-License-Identifier: BSD-3-Clause
 */
package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"runtime/debug"

	"intel/isecl/go-trust-agent/v4/common"
	"intel/isecl/go-trust-agent/v4/config"
	"intel/isecl/go-trust-agent/v4/constants"
	"io/ioutil"
	stdlog "log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/intel-secl/intel-secl/v4/pkg/clients"
	"github.com/intel-secl/intel-secl/v4/pkg/lib/common/auth"
	commContext "github.com/intel-secl/intel-secl/v4/pkg/lib/common/context"
	"github.com/intel-secl/intel-secl/v4/pkg/lib/common/crypt"
	commLog "github.com/intel-secl/intel-secl/v4/pkg/lib/common/log"
	"github.com/intel-secl/intel-secl/v4/pkg/lib/common/log/message"
	"github.com/intel-secl/intel-secl/v4/pkg/lib/common/middleware"
	ct "github.com/intel-secl/intel-secl/v4/pkg/model/aas"
	"github.com/pkg/errors"
)

const (
	getAIKPerm             = "aik:retrieve"
	getAIKCAPerm           = "aik_ca:retrieve"
	getBindingKeyPerm      = "binding_key:retrieve"
	getDAAPerm             = "daa:retrieve"
	getHostInfoPerm        = "host_info:retrieve"
	postDeployManifestPerm = "deploy_manifest:create"
	postAppMeasurementPerm = "application_measurement:create"
	postDeployTagPerm      = "deploy_tag:create"
	postQuotePerm          = "quote:create"
)

type trustAgentWebService struct {
	webParameters WebParameters
	router        *mux.Router
	server        *http.Server
}

type privilegeError struct {
	StatusCode int
	Message    string
}

func (e privilegeError) Error() string {
	log.Trace("resource/service:Error() Entering")
	defer log.Trace("resource/service:Error() Leaving")
	return fmt.Sprintf("%d: %s", e.StatusCode, e.Message)
}

var cacheTime, _ = time.ParseDuration(constants.JWTCertsCacheTime)
var seclog = commLog.GetSecurityLogger()

func newWebService(webParameters *WebParameters, requestHandler common.RequestHandler) (TrustAgentService, error) {
	log.Trace("resource/service:NewTrustAgentHttpService() Entering")
	defer log.Trace("resource/service:NewTrustAgentHttpService() Leaving")

	if webParameters.Port == 0 {
		return nil, errors.New("Port cannot be zero")
	}

	trustAgentService := trustAgentWebService{
		webParameters: *webParameters,
	}

	// Register routes...
	trustAgentService.router = mux.NewRouter()
	// ISECL-8715 - Prevent potential open redirects to external URLs
	trustAgentService.router.SkipClean(true)

	noAuthRouter := trustAgentService.router.PathPrefix("/v2/").Subrouter()
	noAuthRouter.HandleFunc("/version", errorHandler(getVersion())).Methods("GET")

	// use permission-based access control for webservices
	authRouter := trustAgentService.router.PathPrefix("/v2/").Subrouter()
	authRouter.Use(middleware.NewTokenAuth(webParameters.TrustedJWTSigningCertsDir, webParameters.TrustedCaCertsDir, fnGetJwtCerts, cacheTime))

	authRouter.HandleFunc("/aik", errorHandler(requiresPermission(getAik(requestHandler), []string{getAIKPerm}))).Methods("GET")
	authRouter.HandleFunc("/host", errorHandler(requiresPermission(getPlatformInfo(requestHandler), []string{getHostInfoPerm}))).Methods("GET")
	authRouter.HandleFunc("/tpm/quote", errorHandler(requiresPermission(getTpmQuote(requestHandler), []string{postQuotePerm}))).Methods("POST")
	authRouter.HandleFunc("/binding-key-certificate", errorHandler(requiresPermission(getBindingKeyCertificate(requestHandler), []string{getBindingKeyPerm}))).Methods("GET")
	authRouter.HandleFunc("/tag", errorHandler(requiresPermission(setAssetTag(requestHandler), []string{postDeployTagPerm}))).Methods("POST")
	authRouter.HandleFunc("/host/application-measurement", errorHandler(requiresPermission(getApplicationMeasurement(requestHandler), []string{postAppMeasurementPerm}))).Methods("POST")
	authRouter.HandleFunc("/deploy/manifest", errorHandler(requiresPermission(deployManifest(requestHandler), []string{postDeployManifestPerm}))).Methods("POST")

	return &trustAgentService, nil
}

func (service *trustAgentWebService) Start() error {
	log.Trace("resource/service:Start() Entering")
	defer log.Trace("resource/service:Start() Leaving")

	tlsconfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
	}

	httpWriter := os.Stderr
	if httpLogFile, err := os.OpenFile(constants.HttpLogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640); err != nil {
		secLog.WithError(err).Errorf("resource/service:Start() %s Failed to open http log file: %s\n", message.AppRuntimeErr, err.Error())
		log.Tracef("resource/service:Start() %+v", err)
	} else {
		defer func() {
			derr := httpLogFile.Close()
			if derr != nil {
				log.WithError(derr).Warn("Error closing file")
			}
		}()
		httpWriter = httpLogFile
	}

	httpLog := stdlog.New(httpWriter, "", 0)
	service.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", service.webParameters.Port),
		Handler:           handlers.RecoveryHandler(handlers.RecoveryLogger(httpLog), handlers.PrintRecoveryStack(true))(handlers.CombinedLoggingHandler(os.Stderr, service.router)),
		ErrorLog:          httpLog,
		TLSConfig:         tlsconfig,
		ReadTimeout:       service.webParameters.ReadTimeout,
		ReadHeaderTimeout: service.webParameters.ReadHeaderTimeout,
		WriteTimeout:      service.webParameters.WriteTimeout,
		IdleTimeout:       service.webParameters.IdleTimeout,
		MaxHeaderBytes:    service.webParameters.MaxHeaderBytes,
	}

	// dispatch web server go routine
	go func() {
		if err := service.server.ListenAndServeTLS(service.webParameters.TLSCertFilePath, service.webParameters.TLSKeyFilePath); err != nil {
			secLog.Errorf("tasks/service:Start() %s", message.TLSConnectFailed)
			secLog.WithError(err).Fatalf("server:startServer() Failed to start HTTPS server: %s\n", err.Error())
			log.Tracef("%+v", err)
		}
	}()
	secLog.Info(message.ServiceStart)
	secLog.Infof("TrustAgent service is running: %d", service.webParameters.Port)
	log.Infof("TrustAgent service is running: %d", service.webParameters.Port)

	return nil
}

func (service *trustAgentWebService) Stop() error {
	if service.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := service.server.Shutdown(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to gracefully shutdown webserver: %v\n", err)
			log.WithError(err).Info("Failed to gracefully shutdown webserver")
			return err
		}
	}

	return nil
}

// requiresPermission checks the JWT in the request for the required access permissions
func requiresPermission(eh endpointHandler, permissionNames []string) endpointHandler {
	log.Trace("resource/service:requiresPermission() Entering")
	defer log.Trace("resource/service:requiresPermission() Leaving")
	return func(w http.ResponseWriter, r *http.Request) error {
		privileges, err := commContext.GetUserPermissions(r)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			_, writeErr := w.Write([]byte("Could not get user roles from http context"))
			if writeErr != nil {
				log.WithError(writeErr).Warn("resource/service:requiresPermission() Error while writing response")
			}
			secLog.Errorf("resource/service:requiresPermission() %s Roles: %v | Context: %v", message.AuthenticationFailed, permissionNames, r.Context())
			return errors.Wrap(err, "resource/service:requiresPermission() Could not get user roles from http context")
		}
		reqPermissions := ct.PermissionInfo{Service: constants.TAServiceName, Rules: permissionNames}

		_, foundMatchingPermission := auth.ValidatePermissionAndGetPermissionsContext(privileges, reqPermissions,
			true)
		if !foundMatchingPermission {
			w.WriteHeader(http.StatusUnauthorized)
			secLog.Errorf("resource/service:requiresPermission() %s Insufficient privileges to access %s", message.UnauthorizedAccess, r.RequestURI)
			return &privilegeError{Message: "Insufficient privileges to access " + r.RequestURI, StatusCode: http.StatusUnauthorized}
		}
		w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		secLog.Debugf("resource/service:requiresPermission() %s - %s", message.AuthorizedAccess, r.RequestURI)
		return eh(w, r)
	}
}

// endpointHandler is the same as http.ResponseHandler, but returns an error that can be handled by a generic
// middleware handler
type endpointHandler func(w http.ResponseWriter, r *http.Request) error

func errorHandler(eh endpointHandler) http.HandlerFunc {
	log.Trace("resource/service:errorHandler() Entering")
	defer log.Trace("resource/service:errorHandler() Leaving")
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf("Panic occurred: %+v\n%s", err, string(debug.Stack()))
				http.Error(w, "Unknown Error", http.StatusInternalServerError)
			}
		}()

		if err := eh(w, r); err != nil {
			if strings.TrimSpace(strings.ToLower(err.Error())) == "record not found" {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			switch t := err.(type) {
			case *common.EndpointError:
				http.Error(w, t.Message, t.StatusCode)
			case privilegeError:
				http.Error(w, t.Message, t.StatusCode)
			default:
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	}
}

func fnGetJwtCerts() error {
	log.Trace("resource/service:fnGetJwtCerts() Entering")
	defer log.Trace("resource/service:fnGetJwtCerts() Leaving")

	cfg, err := config.NewConfigFromYaml(constants.ConfigFilePath)
	if err != nil {
		fmt.Printf("ERROR: %+v\n", err)
		return nil
	}

	aasURL := cfg.AAS.BaseURL

	url := aasURL + "jwt-certificates"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("accept", "application/x-pem-file")
	secLog.Debugf("resource/service::fnGetJwtCerts() Connecting to AAS Endpoint %s", url)

	caCerts, err := crypt.GetCertsFromDir(constants.TrustedCaCertsDir)
	if err != nil {
		log.WithError(err).Errorf("resource/service::fnGetJwtCerts() Error while getting certs from %s", constants.TrustedCaCertsDir)
		return errors.Wrap(err, "resource/service::fnGetJwtCerts() Error while getting certs from %s")
	}

	hc, err := clients.HTTPClientWithCA(caCerts)
	if err != nil {
		return errors.Wrap(err, "resource/service:fnGetJwtCerts() Error setting up HTTP client")
	}

	res, err := hc.Do(req)
	if err != nil {
		return errors.Wrap(err, "resource/service:fnGetJwtCerts() Could not retrieve jwt certificate")
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "resource/service:fnGetJwtCerts() Error while reading response body")
	}

	err = crypt.SavePemCertWithShortSha1FileName(body, constants.TrustedJWTSigningCertsDir)
	if err != nil {
		return errors.Wrap(err, "resource/service:fnGetJwtCerts() Error while saving certificate")
	}

	return nil
}
