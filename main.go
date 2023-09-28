package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"sync"
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

func downloadPackage(packageName string, version string, wg *sync.WaitGroup, ch chan string) {
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

	ch <- packageName
	wg.Done()
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

func dependencyTreeMT(packageName string, version string) (map[string]string, error) {
	depTree := map[string]string{}
	depTreeMutex := sync.Mutex{}
	var wg sync.WaitGroup

	var recursiveSearch func(packageName string, version string)
	recursiveSearch = func(packageName string, version string) {
		defer wg.Done()

		deps, err := getDependencies(packageName, version)
		if err != nil {
			fmt.Println("[ERROR] ", err)
			return
		}

		if len(deps) > 0 {
			for package_, version := range deps {
				depTreeMutex.Lock()
				if _, exists := depTree[package_]; !exists {
					depTree[package_] = version
					wg.Add(1)
					go recursiveSearch(package_, extractSemver(version))
				}
				depTreeMutex.Unlock()
			}
		}
	}

	wg.Add(1)
	go recursiveSearch(packageName, version)
	wg.Wait()

	return depTree, nil
}

func extractSemver(input string) string {
	if input == "latest" {
		return input
	}
	regexpPattern := "([0-9]+\\.[0-9]+\\.[0-9]+)"
	re := regexp.MustCompile(regexpPattern)
	match := re.FindString(input)

	return match
}

func downloadPackageFull(pkgName string, version string) {
	depTree, err := dependencyTreeMT(pkgName, extractSemver(version))

	if err != nil {
		fmt.Println("[ERROR] ", err)
	}

	var wg sync.WaitGroup
	ch := make(chan string)

	for package_, ver := range depTree {
		wg.Add(1)
		go downloadPackage(package_, extractSemver(ver), &wg, ch)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for pkgName := range ch {
		fmt.Println("Finished installing: ", pkgName)
	}
}

func main() {
	start := time.Now()
	downloadPackageFull("express", "latest")
	fmt.Println("Time taken: ", time.Since(start).Seconds())
}
