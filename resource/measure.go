/*
* Copyright (C) 2020 Intel Corporation
* SPDX-License-Identifier: BSD-3-Clause
 */
package resource

import (
	"bytes"
	"encoding/xml"

	"intel/isecl/go-trust-agent/v4/common"
	"io/ioutil"
	"net/http"

	"github.com/intel-secl/intel-secl/v4/pkg/lib/common/log/message"
	taModel "github.com/intel-secl/intel-secl/v4/pkg/model/ta"
)

// Uses /opt/tbootxml/bin/measure to measure the supplied manifest
func getApplicationMeasurement(requestHandler common.RequestHandler) endpointHandler {
	return func(httpWriter http.ResponseWriter, httpRequest *http.Request) error {
		log.Trace("resource/measure:getApplicationMeasurement() Entering")
		defer log.Trace("resource/measure:getApplicationMeasurement() Leaving")

		log.Debugf("resource/measure:getApplicationMeasurement() Request: %s", httpRequest.URL.Path)

		contentType := httpRequest.Header.Get("Content-Type")
		if contentType != "application/xml" {
			log.Errorf("resource/measure:getApplicationMeasurement() %s - Invalid content-type '%s'", message.InvalidInputBadParam, contentType)
			return &common.EndpointError{Message: "Invalid content-type", StatusCode: http.StatusBadRequest}
		}

		// receive a manifest from hvs in the request body
		manifestXml, err := ioutil.ReadAll(httpRequest.Body)
		if err != nil {
			seclog.WithError(err).Errorf("resource/measure:getApplicationMeasurement() %s - Error reading manifest xml", message.InvalidInputBadParam)
			return &common.EndpointError{Message: "Error reading manifest xml", StatusCode: http.StatusBadRequest}
		}

		// make sure the xml is well formed, all other validation will be
		// peformed by 'measure' cmd line below
		manifest := taModel.Manifest{}
		err = xml.Unmarshal(manifestXml, &manifest)
		if err != nil {
			secLog.WithError(err).Errorf("resource/measure:getApplicationMeasurement() %s - Invalid xml format", message.InvalidInputBadParam)
			return &common.EndpointError{Message: "Error: Invalid XML format", StatusCode: http.StatusBadRequest}
		}

		measurement, err := requestHandler.GetApplicationMeasurement(&manifest)
		if err != nil {
			log.WithError(err).Errorf("resource/measure:getApplicationMeasurement() %s - Error getting measurement", message.AppRuntimeErr)
			return err
		}

		measureBytes, err := xml.Marshal(measurement)
		if err != nil {
			secLog.WithError(err).Errorf("resource/measure:getApplicationMeasurement() %s - Invalid xml format", message.InvalidInputBadParam)
			return &common.EndpointError{Message: "Error: Invalid XML format of generated XML", StatusCode: http.StatusInternalServerError}
		}

		httpWriter.WriteHeader(http.StatusOK)
		_, _ = bytes.NewBuffer(measureBytes).WriteTo(httpWriter)
		return nil
	}
}
