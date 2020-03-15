// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package proxy

import (
	"fmt"
	"net/http"

	"testing"

	"regexp"

	"github.com/ngduchai/faas-cli/stack"
	"github.com/ngduchai/faas-cli/test"
)

const tlsNoVerify = true

type deployProxyTest struct {
	title               string
	mockServerResponses []int
	replace             bool
	update              bool
	expectedOutput      string
}

func runDeployProxyTest(t *testing.T, deployTest deployProxyTest) {
	s := test.MockHttpServerStatus(
		t,
		deployTest.mockServerResponses...,
	)
	defer s.Close()

	stdout := test.CaptureStdout(func() {
		DeployFunction(&DeployFunctionSpec{
			"fproces",
			s.URL,
			"function",
			"image",
			"dXNlcjpwYXNzd29yZA==",
			"language",
			deployTest.replace,
			nil,
			"network",
			[]string{},
			deployTest.update,
			[]string{},
			map[string]string{},
			map[string]string{},
			FunctionResourceRequest{},
			false,
			tlsNoVerify,
			0,
			&stack.FunctionResources{},
			10,
		})
	})

	r := regexp.MustCompile(deployTest.expectedOutput)
	if !r.MatchString(stdout) {
		t.Fatalf("Output not matched: %s", stdout)
	}
}

func Test_RunDeployProxyTests(t *testing.T) {
	var deployProxyTests = []deployProxyTest{
		{
			title:               "200_Deploy",
			mockServerResponses: []int{http.StatusOK, http.StatusOK},
			replace:             true,
			update:              false,
			expectedOutput:      `(?m:Deployed)`,
		},
		{
			title:               "404_Deploy",
			mockServerResponses: []int{http.StatusOK, http.StatusNotFound},
			replace:             true,
			update:              false,
			expectedOutput:      `(?m:Unexpected status: 404)`,
		},
		{
			title:               "UpdateFailedDeployed",
			mockServerResponses: []int{http.StatusNotFound, http.StatusOK},
			replace:             false,
			update:              true,
			expectedOutput:      `(?m:Deployed)`,
		},
	}
	for _, tst := range deployProxyTests {
		t.Run(tst.title, func(t *testing.T) {
			runDeployProxyTest(t, tst)
		})
	}
}

func Test_DeployFunction_MissingURLPrefix(t *testing.T) {
	url := "127.0.0.1:8080"

	stdout := test.CaptureStdout(func() {
		DeployFunction(&DeployFunctionSpec{
			"fprocess",
			url,
			"function",
			"image",
			"dXNlcjpwYXNzd29yZA==",
			"language",
			false,
			nil,
			"network",
			[]string{},
			false,
			[]string{},
			map[string]string{},
			map[string]string{},
			FunctionResourceRequest{},
			false,
			tlsNoVerify,
			0.0,
			&stack.FunctionResources{},
			10,
		})
	})

	expectedErrMsg := "first path segment in URL cannot contain colon"
	r := regexp.MustCompile(fmt.Sprintf("(?m:%s)", expectedErrMsg))
	if !r.MatchString(stdout) {
		t.Fatalf("Want: %s\nGot: %s", expectedErrMsg, stdout)
	}
}
