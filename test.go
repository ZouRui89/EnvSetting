package main

import (
	"fmt"
	"os/exec"
)

func main()  {
	cmd := exec.Command("git", "clone", "https://github.com/ZouRui89/EnvSetting")
	if err := cmd.Run(); err != nil {
		fmt.Println("clone error: %v", err)
	}
}