package main

import (
	"fmt"
	"log"
	"path"
	"sync"
	"testing"
)

func TestGenerator(t *testing.T) {
	packageNames := []string{"", "cluster", "project"}

	wg := sync.WaitGroup{}
	wg.Add(len(packageNames))

	for _, packageName := range packageNames {
		go func(pkgName string) {
			defer wg.Done()

			schemasFilePath := path.Join("schemas", pkgName, "schemas.json")
			fmt.Println(schemasFilePath)

			err := generateFiles(schemasFilePath, pkgName)
			if err != nil {
				log.Fatal(err)
			}
		}(packageName)
	}

	wg.Wait()
}
