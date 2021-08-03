package main

import (
	"fmt"
	"log"
	"path"
	"testing"
)

func TestGenerator(t *testing.T) {
	useLombok := false
	packageName := "cluster"
	schemasFilePath := path.Join("schemas", packageName, "schemas.json")
	fmt.Println(schemasFilePath)

	err := generateFiles(schemasFilePath, packageName, useLombok)
	if err != nil {
		log.Fatal(err)
	}
}

func TestGeneratorWithLombok(t *testing.T) {
	useLombok := true
	packageName := "cluster"
	schemasFilePath := path.Join("schemas", packageName, "schemas.json")
	fmt.Println(schemasFilePath)

	err := generateFiles(schemasFilePath, packageName, useLombok)
	if err != nil {
		log.Fatal(err)
	}
}
