package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	// "github.com/Masterminds/semver/v3"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type DependencyResponse struct {
	Dependencies map[string]string `json:"dependencies"`
}

func getDependencies(packageName string, version string) (map[string]string, error) {
	url := "https://registry.npmjs.org/" + packageName + "/" + version
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	var depResponse DependencyResponse

	if err := json.Unmarshal(body, &depResponse); err != nil {
		return nil, err
	}

	return depResponse.Dependencies, nil
}

func downloadPackage(packageName string, version string) {
	_, exists := os.Stat("node_modules")

	if os.IsNotExist(exists) {
		err := os.Mkdir("node_modules", 0755)
		if err != nil {
			fmt.Println("[ERROR] ", err)
			return
		}
	}

	err := os.Mkdir("node_modules/"+packageName, 0755)
	if err != nil {
		fmt.Println("[ERROR] ", err)
		return
	}

	url := "https://registry.npmjs.org/" + packageName + "/-/" + packageName + "-" + version + ".tgz"

	response, err := http.Get(url)

	if err != nil {
		fmt.Println("[ERROR] ", err)
		return
	}

	defer response.Body.Close()

	tarball, _ := io.ReadAll(response.Body)

	tarballReader, err := gzip.NewReader(bytes.NewReader(tarball))

	if err != nil {
		fmt.Println("[ERROR] ", err)
		return
	}
	defer tarballReader.Close()

	tarReader := tar.NewReader(tarballReader)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			fmt.Println("[ERROR] ", err)
			return
		}

		name_split := strings.Split(header.Name, "/")

		path := "node_modules/" + packageName + "/"

		for _, dirName := range name_split[1 : len(name_split)-1] {
			path += dirName + "/"

			_, exists := os.Stat(path)

			if os.IsNotExist(exists) {
				err := os.Mkdir(path, 0755)
				if err != nil {
					fmt.Println("[ERROR] ", err)
					return
				}
			}
		}

		file, err := os.Create(path + name_split[len(name_split)-1])

		if err != nil {
			fmt.Println("[ERROR] ", err)
			return
		}
		defer file.Close()

		_, err = io.Copy(file, tarReader)

		if err != nil {
			fmt.Println("[ERROR] ", err)
			return
		}
	}
}

func dependencyTree(packageName string, version string) (map[string]string, error) {
	depTree := map[string]string{}

	var recursiveSearch func(packageName string, version string)
	recursiveSearch = func(packageName string, version string) {
		deps, _ := getDependencies(packageName, version)

		if len(deps) > 0 {
			for package_, version := range deps {
				if _, exists := depTree[package_]; !exists {
					depTree[package_] = version
					recursiveSearch(package_, extractSemver(version))
				}
			}
		}
	}

	recursiveSearch(packageName, version)

	return depTree, nil
}

func extractSemver(input string) string {
	regexpPattern := "([0-9]+\\.[0-9]+\\.[0-9]+)"
	re := regexp.MustCompile(regexpPattern)
	match := re.FindString(input)

	return match
}

func main() {
	// downloadPackage("express", "4.18.2")
	start := time.Now()
	depTree, err := dependencyTree("express", "latest")

	if err != nil {
		fmt.Println("[ERROR] ", err)
		return
	}

	elapsed := time.Since(start)

	fmt.Println("Dep Tree generation time: ", elapsed.Seconds())

	start = time.Now()

	for package_, version := range depTree {
		downloadPackage(package_, extractSemver(version))
		fmt.Printf("%s@%s Installed!\n", package_, extractSemver(version))
	}

	fmt.Println("Time to install express: ", elapsed.Seconds())
}
