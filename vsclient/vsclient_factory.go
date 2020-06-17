/*
 * Copyright (C) 2020 Intel Corporation
 * SPDX-License-Identifier: BSD-3-Clause
 */
package vsclient

import (
	"intel/isecl/go-trust-agent/v2/constants"
	"intel/isecl/lib/clients/v2"
	"errors"
	"net/http"
	"net/url"
)

type VSClientFactory interface {
	HostsClient() (HostsClient, error)
	FlavorsClient() (FlavorsClient, error)
	ManifestsClient() (ManifestsClient, error)
	PrivacyCAClient() (PrivacyCAClient, error)
}

type vsClientConfig struct {
	// BaseURL specifies the URL base for the HVS, for example https://hvs.server:8443/v2
	BaseURL string
	// BearerToken is the JWT token required for authentication with external services
	BearerToken string
}

func NewVSClientFactory(baseURL string, bearerToken string) (VSClientFactory, error) {

	cfg := vsClientConfig {BaseURL: baseURL, BearerToken: bearerToken}

	defaultFactory := defaultVSClientFactory{&cfg}
	return &defaultFactory, nil
}

//-------------------------------------------------------------------------------------------------
// Implementation
//-------------------------------------------------------------------------------------------------

type defaultVSClientFactory struct {
	cfg *vsClientConfig
}

func (vsClientFactory *defaultVSClientFactory) FlavorsClient() (FlavorsClient, error) {
	httpClient, err := vsClientFactory.createHttpClient()
	if err != nil {
		return nil, err
	}

	return &flavorsClientImpl{httpClient, vsClientFactory.cfg}, nil
}

func (vsClientFactory *defaultVSClientFactory) HostsClient() (HostsClient, error) {
	httpClient, err := vsClientFactory.createHttpClient()
	if err != nil {
		return nil, err
	}

	return &hostsClientImpl{httpClient, vsClientFactory.cfg}, nil
}

func (vsClientFactory *defaultVSClientFactory) ManifestsClient() (ManifestsClient, error) {
	httpClient, err := vsClientFactory.createHttpClient()
	if err != nil {
		return nil, err
	}

	return &manifestsClientImpl{httpClient, vsClientFactory.cfg}, nil
}

func (vsClientFactory *defaultVSClientFactory) PrivacyCAClient() (PrivacyCAClient, error) {
	httpClient, err := vsClientFactory.createHttpClient()
	if err != nil {
		return nil, err
	}

	return &privacyCAClientImpl{httpClient, vsClientFactory.cfg}, nil
}

func (vsClientFactory *defaultVSClientFactory) createHttpClient() (*http.Client, error) {
	log.Trace("vsclient/vsclient_factory:createHttpClient() Entering")
	defer log.Trace("vsclient/vsclient_factory:createHttpClient() Leaving")

	_, err := url.ParseRequestURI(vsClientFactory.cfg.BaseURL)
	if err != nil {
		return nil, err
	}

	if vsClientFactory.cfg.BearerToken == "" {
		return nil, errors.New("The bearer token is empty")
	}

	// Here we need to return a client which has validated the HVS TLS cert-chain
	client, err := clients.HTTPClientWithCADir(constants.TrustedCaCertsDir)
	if err != nil {
		log.WithError(err).Error("vsclient/vsclient_factory:createHttpClient() Error while creating http client")
		return nil, err
	}

	return &http.Client{Transport: client.Transport}, nil
}
