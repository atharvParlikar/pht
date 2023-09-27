package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type DependencyResponse struct {
	Dependencies map[string]string `json:"dependencies"`
}

func getDependencies(packageName string, version string) (map[string]string, error) {
	url := "https://registry.npmjs.org/" + packageName + "/" + version
	fmt.Println("[GET] ", url)
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

	url := "https://registry.npmjs.org/" + packageName + "/-/" + packageName + "-" + version + ".tgz"

	fmt.Println("URL: ", url)

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

		path := "node_modules/"

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

func main() {
	downloadPackage("express", "4.18.2")
	// dep, err := getDependencies("express", "4.0.0")
	// if err != nil {
	// 	fmt.Println("Error: ", err)
	// 	return
	// }

	// for package_, version := range dep {
	// 	fmt.Printf("%s: %s\n", package_, version)
	// }
}
