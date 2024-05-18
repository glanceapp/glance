package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
)

// bunch of spaget but it does the job for now
// TODO: tidy up and add a proper build system with CI

const buildPath = "./build"
const archivesPath = "./build/archives"
const executableName = "glance"
const ownerAndRepo = "glanceapp/glance"
const moduleName = "github.com/" + ownerAndRepo

type archiveType int

const (
	archiveTypeTarGz archiveType = iota
	archiveTypeZip
)

type buildInfo struct {
	version string
}

type buildTarget struct {
	os        string
	arch      string
	armV      int
	extension string
	archive   archiveType
}

var buildTargets = []buildTarget{
	{
		os:        "windows",
		arch:      "amd64",
		extension: ".exe",
		archive:   archiveTypeZip,
	},
	{
		os:        "windows",
		arch:      "arm64",
		extension: ".exe",
		archive:   archiveTypeZip,
	},
	{
		os:   "linux",
		arch: "amd64",
	},
	{
		os:   "linux",
		arch: "arm64",
	},
	{
		os:   "linux",
		arch: "arm",
		armV: 6,
	},
	{
		os:   "linux",
		arch: "arm",
		armV: 7,
	},
	{
		os:   "openbsd",
		arch: "amd64",
	},
	{
		os:   "openbsd",
		arch: "386",
	},
}

func hasUncommitedChanges() (bool, error) {
	output, err := exec.Command("git", "status", "--porcelain").CombinedOutput()

	if err != nil {
		return false, err
	}

	return len(output) > 0, nil
}

func main() {
	flags := flag.NewFlagSet("", flag.ExitOnError)

	specificTag := flags.String("tag", "", "Which tagged version to build")

	err := flags.Parse(os.Args[1:])

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	uncommitedChanges, err := hasUncommitedChanges()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if uncommitedChanges {
		fmt.Println("There are uncommited changes - commit, stash or discard them first")
		os.Exit(1)
	}

	cwd, err := os.Getwd()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	_, err = os.Stat(buildPath)

	if err == nil {
		fmt.Println("Cleaning up build path")
		os.RemoveAll(buildPath)
	}

	os.Mkdir(buildPath, 0755)
	os.Mkdir(archivesPath, 0755)

	var version string

	if *specificTag == "" {
		version, err := getVersionFromGit()

		if err != nil {
			fmt.Println(version, err)
			os.Exit(1)
		}
	} else {
		version = *specificTag
	}

	output, err := exec.Command("git", "checkout", "tags/"+version).CombinedOutput()

	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
		os.Exit(1)
	}

	info := buildInfo{
		version: version,
	}

	for _, target := range buildTargets {
		fmt.Printf("Building for %s/%s\n", target.os, target.arch)
		if err := build(cwd, info, target); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	versionTag := fmt.Sprintf("%s:%s", ownerAndRepo, version)
	latestTag := fmt.Sprintf("%s:latest", ownerAndRepo)

	fmt.Println("Building docker image")

	var dockerBuildOptions = []string{
		"docker", "build",
		"--platform=linux/amd64,linux/arm64,linux/arm/v7",
		"-t", versionTag,
	}

	if !strings.Contains(version, "beta") {
		dockerBuildOptions = append(dockerBuildOptions, "-t", latestTag)
	}

	dockerBuildOptions = append(dockerBuildOptions, ".")

	output, err = exec.Command("sudo", dockerBuildOptions...).CombinedOutput()

	if err != nil {
		fmt.Println(string(output))
		fmt.Println(err)
		os.Exit(1)
	}

	var input string
	fmt.Print("Push docker image? [y/n]: ")
	fmt.Scanln(&input)

	if input != "y" {
		os.Exit(0)
	}

	output, err = exec.Command(
		"sudo", "docker", "push", versionTag,
	).CombinedOutput()

	if err != nil {
		fmt.Printf("Failed pushing %s:\n", versionTag)
		fmt.Println(string(output))
		fmt.Println(err)
		os.Exit(1)
	}

	if strings.Contains(version, "beta") {
		return
	}

	output, err = exec.Command(
		"sudo", "docker", "push", latestTag,
	).CombinedOutput()

	if err != nil {
		fmt.Printf("Failed pushing %s:\n", latestTag)
		fmt.Println(string(output))
		fmt.Println(err)
		os.Exit(1)
	}
}

func getVersionFromGit() (string, error) {
	output, err := exec.Command("git", "describe", "--tags", "--abbrev=0").CombinedOutput()

	if err == nil {
		return strings.TrimSpace(string(output)), err
	}

	return string(output), err
}

func archiveFile(name string, target string, t archiveType) error {
	var output []byte
	var err error

	if t == archiveTypeZip {
		output, err = exec.Command("zip", "-j", path.Join(archivesPath, name+".zip"), target).CombinedOutput()
	} else if t == archiveTypeTarGz {
		output, err = exec.Command("tar", "-C", buildPath, "-czf", path.Join(archivesPath, name+".tar.gz"), name).CombinedOutput()
	}

	if err != nil {
		fmt.Println(string(output))
		return err
	}

	return nil
}

func build(workingDir string, info buildInfo, target buildTarget) error {
	var name string

	if target.arch != "arm" {
		name = fmt.Sprintf("%s-%s-%s%s", executableName, target.os, target.arch, target.extension)
	} else {
		name = fmt.Sprintf("%s-%s-%sv%d", executableName, target.os, target.arch, target.armV)
	}

	binaryPath := path.Join(buildPath, name)

	glancePackage := moduleName + "/internal/glance"

	flags := "-s -w"
	flags += fmt.Sprintf(" -X %s.buildVersion=%s", glancePackage, info.version)

	cmd := exec.Command(
		"go",
		"build",
		"--trimpath",
		"--ldflags",
		flags,
		"-o",
		binaryPath,
	)

	cmd.Dir = workingDir
	env := append(os.Environ(), "GOOS="+target.os, "GOARCH="+target.arch, "CGO_ENABLED=0")

	if target.arch == "arm" {
		env = append(env, fmt.Sprintf("GOARM=%d", target.armV))
	}

	cmd.Env = env
	output, err := cmd.CombinedOutput()

	if err != nil {
		fmt.Println(err)
		fmt.Println(string(output))
		return err
	}

	os.Chmod(binaryPath, 0755)

	fmt.Println("Creating archive")
	if err := archiveFile(name, binaryPath, target.archive); err != nil {
		return err
	}

	return nil
}
