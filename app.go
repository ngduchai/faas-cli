// Copyright (c) Alex Ellis 2017. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"net/http"

	"encoding/json"

	"bytes"
	"os"

	"net/url"

	"github.com/alexellis/faas/gateway/requests"
)

const providerName = "faas"
const defaultNetwork = "func_functions"

func main() {
	// var handler string
	var handler string
	var image string

	var action string
	var functionName string
	var gateway string
	var fprocess string
	var language string
	var replace bool
	var nocache bool
	var yamlFile string
	var yamlFileShort string

	flag.StringVar(&handler, "handler", "", "handler for function, i.e. handler.js")
	flag.StringVar(&image, "image", "", "Docker image name to build")
	flag.StringVar(&action, "action", "", "either build or deploy")
	flag.StringVar(&functionName, "name", "", "give the name of your deployed function")
	flag.StringVar(&gateway, "gateway", "http://localhost:8080", "gateway URI - i.e. http://localhost:8080")
	flag.StringVar(&fprocess, "fprocess", "", "fprocess to be run by the watchdog")
	flag.StringVar(&language, "lang", "node", "programming language template, default is: node")
	flag.BoolVar(&replace, "replace", true, "replace any existing function")
	flag.BoolVar(&nocache, "no-cache", false, "do not use Docker's build cache")

	flag.StringVar(&yamlFile, "yaml", "", "use a yaml file for a set of functions")
	flag.StringVar(&yamlFileShort, "f", "", "use a yaml file for a set of functions (same as -yaml)")

	flag.Parse()

	// support short-argument -f
	if len(yamlFile) == 0 && len(yamlFileShort) > 0 {
		yamlFile = yamlFileShort
	}

	var services Services
	if len(yamlFile) > 0 {
		var err error
		var fileData []byte
		urlParsed, err := url.Parse(yamlFile)
		if err == nil && len(urlParsed.Scheme) > 0 {
			fmt.Println("Parsed: " + urlParsed.String())
			fileData, err = fetchYaml(urlParsed)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
		} else {
			fileData, err = ioutil.ReadFile(yamlFile)
			if err != nil {
				fmt.Printf("Error: %s\n", err.Error())
				return
			}
		}

		err = yaml.Unmarshal(fileData, &services)
		if err != nil {
			fmt.Printf("Error with YAML file: %s\n", err.Error())
			return
		}
		if services.Provider.Name != providerName {
			fmt.Printf("'%s' is the only valid provider for this tool - found: %s.\n", providerName, services.Provider.Name)
			return
		}
	}

	if len(action) == 0 {
		fmt.Println("give either -action= build or deploy")
		return
	}

	switch action {
	case "build":
		if len(services.Functions) > 0 {
			for k, function := range services.Functions {
				function.Name = k
				// fmt.Println(k, function)
				fmt.Printf("Building: %s.\n", function.Name)
				buildImage(function.Image, function.Handler, function.Name, function.Language, nocache)
			}
		} else {
			if len(image) == 0 {
				fmt.Println("Give a valid -image name for your Docker image.")
				return
			}
			if len(handler) == 0 {
				fmt.Println("Please give the full path to your function's handler.")
				return
			}
			if len(functionName) == 0 {
				fmt.Println("Please give the deployed -name of your function")
				return
			}
			buildImage(image, handler, functionName, language, nocache)
		}
		break
	case "deploy":
		if len(services.Functions) > 0 {
			if len(services.Provider.Network) == 0 {
				services.Provider.Network = defaultNetwork
			}

			for k, function := range services.Functions {
				function.Name = k
				// fmt.Println(k, function)
				fmt.Printf("Deploying: %s.\n", function.Name)

				deployFunction(function.FProcess, services.Provider.GatewayURL, function.Name, function.Image, function.Language, replace, function.Environment, services.Provider.Network)
			}
		} else {
			if len(image) == 0 {
				fmt.Println("Give an image name to be deployed.")
				return
			}
			if len(functionName) == 0 {
				fmt.Println("Give a -name for your function as it will be deployed on FaaS")
				return
			}

			deployFunction(fprocess, gateway, functionName, image, language, replace, map[string]string{}, defaultNetwork)
		}
		break
	case "push":
		if len(services.Functions) > 0 {
			for k, function := range services.Functions {
				function.Name = k
				fmt.Printf("Pushing: %s to remote repository.\n", function.Name)
				pushImage(function.Image)
			}
		} else {
			fmt.Println("The '-action push' flag only works with a YAML file.")
			return
		}
	default:
		fmt.Println("-action must be 'build', 'deploy' or 'push'.")
		break
	}
}

func deployFunction(fprocess string, gateway string, functionName string, image string, language string, replace bool, envVars map[string]string, network string) {

	// Need to alter Gateway to allow nil/empty string as fprocess, to avoid this repetition.
	fprocessTemplate := "node index.js"
	if len(fprocess) > 0 {
		fprocessTemplate = fprocess
	}
	if language == "python" {
		fprocessTemplate = "python index.py"
	}

	if replace {
		deleteFunction(gateway, functionName)
	}

	// TODO: allow registry auth to be specified or read from local Docker credentials store
	req := requests.CreateFunctionRequest{
		EnvProcess: fprocessTemplate,
		Image:      image,
		Network:    "func_functions", // todo: specify network as an override
		Service:    functionName,
		EnvVars:    envVars,
	}

	reqBytes, _ := json.Marshal(&req)
	reader := bytes.NewReader(reqBytes)
	res, err := http.Post(gateway+"/system/functions", "application/json", reader)
	if err != nil {
		fmt.Println("Is FaaS deployed? Do you need to specify the -gateway flag?")
		fmt.Println(err)
		return
	}

	fmt.Println(res.Status)
	deployedURL := fmt.Sprintf("URL: %s/function/%s\n", gateway, functionName)
	fmt.Println(deployedURL)
}

func deleteFunction(gateway string, functionName string) {
	delReq := requests.DeleteFunctionRequest{FunctionName: functionName}
	reqBytes, _ := json.Marshal(&delReq)
	reader := bytes.NewReader(reqBytes)

	c := http.Client{}
	req, _ := http.NewRequest("DELETE", gateway+"/system/functions", reader)
	req.Header.Set("Content-Type", "application/json")
	delRes, delErr := c.Do(req)

	if delErr != nil {
		fmt.Printf("Error removing existing function: %s, gateway=%s, functionName=%s\n", delErr.Error(), gateway, functionName)
		return
	}
	switch delRes.StatusCode {
	case 200:
		fmt.Println("Removing old service.")
	case 404:
		fmt.Println("No existing service to remove")
	}
}

func pushImage(image string) {
	execBuild("./", []string{"docker", "push", image})
}

func buildImage(image string, handler string, functionName string, language string, nocache bool) {

	switch language {
	case "node", "python":
		tempPath := createBuildTemplate(functionName, handler, language)

		fmt.Printf("Building: %s with Docker. Please wait..\n", image)

		cacheFlag := ""
		if nocache {
			cacheFlag = " --no-cache"
		}

		builder := strings.Split(fmt.Sprintf("docker build %s-t %s .", cacheFlag, image), " ")
		if len(os.Getenv("http_proxy")) > 0 || len(os.Getenv("http_proxy")) > 0 {
			builder = strings.Split(fmt.Sprintf("docker build %s--build-arg http_proxy=%s --build-arg https_proxy=%s -t %s .", cacheFlag, os.Getenv("http_proxy"), os.Getenv("https_proxy"), image), " ")
		}

		fmt.Println(strings.Join(builder, " "))
		execBuild(tempPath, builder)
	default:
		log.Fatalf("Language template: %s not supported. Build a custom Dockerfile instead.", language)
	}

	fmt.Printf("Image: %s built.\n", image)
}

// createBuildTemplate creates temporary build folder to perform a Docker build with Node template
func createBuildTemplate(functionName string, handler string, language string) string {
	tempPath := fmt.Sprintf("./build/%s/", functionName)
	fmt.Printf("Clearing temporary folder: %s\n", tempPath)

	clearErr := os.RemoveAll(tempPath)
	if clearErr != nil {
		fmt.Printf("Error clearing down temporary build folder %s\n", tempPath)
	}

	fmt.Printf("Preparing %s %s\n", handler+"/", tempPath+"function")

	functionPath := tempPath + "/function"
	mkdirErr := os.MkdirAll(functionPath, 0700)
	if mkdirErr != nil {
		fmt.Printf("Error creating path %s - %s.\n", functionPath, mkdirErr.Error())
	}

	// TODO: index folders and copy everything from template, rather than set folders.
	// Drop in template
	copyFiles("./template/"+language, tempPath)
	copyFiles("./template/"+language+"/function", tempPath+"function/")

	// Overlay in user-function
	copyFiles(handler, tempPath+"function/")

	return tempPath
}

func copyFiles(path string, destination string) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() == false {
			cp(path+"/"+file.Name(), destination+file.Name())
		}
	}
}

func cp(src string, destination string) error {
	fmt.Printf("cp - %s %s\n", src, destination)
	memoryBuffer, readErr := ioutil.ReadFile(src)
	if readErr != nil {
		return fmt.Errorf("Error reading source file: %s\n" + readErr.Error())
	}
	writeErr := ioutil.WriteFile(destination, memoryBuffer, 0660)
	if writeErr != nil {
		return fmt.Errorf("Error writing file: %s\n" + writeErr.Error())
	}

	return nil
}

func execBuild(tempPath string, builder []string) {
	targetCmd := exec.Command(builder[0], builder[1:]...)
	targetCmd.Dir = tempPath
	targetCmd.Stdout = os.Stdout
	targetCmd.Stderr = os.Stderr
	targetCmd.Start()
	targetCmd.Wait()
}

func fetchYaml(address *url.URL) ([]byte, error) {
	req, err := http.NewRequest("GET", address.String(), nil)
	if err != nil {
		return nil, err
	}
	c := http.Client{}
	res, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resBytes, err := ioutil.ReadAll(res.Body)

	return resBytes, err
}
