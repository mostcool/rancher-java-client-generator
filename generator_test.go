package main

import (
	"fmt"
	"log"
	"path"
	"testing"
)

func TestGenerator(t *testing.T) {
	packageName := "cluster"
	schemasFilePath := path.Join("schemas", packageName, "schemas.json")
	fmt.Println(schemasFilePath)

	err := generateFiles(schemasFilePath, packageName)
	if err != nil {
		log.Fatal(err)
	}
}
