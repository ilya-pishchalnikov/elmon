package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

func main() {
    fmt.Println("=== Container execution check ===")
    
    // 1. CGroup check
    fmt.Printf("1. CGroup check: ")
    if isDockerContainerByCGroup() {
        fmt.Println("✅ Container (Docker)")
    } else {
        fmt.Println("❌ Not a container")
    }

    // 2. .dockerenv file check
    fmt.Printf("2. .dockerenv file: ")
    if isDockerContainerByEnvFile() {
        fmt.Println("✅ Container (Docker)")
    } else {
        fmt.Println("❌ Not a container")
    }

    // 3. Hostname check
    fmt.Printf("3. Hostname: ")
    hostname, _ := os.Hostname()
    fmt.Printf("%s - ", hostname)
    if isDockerContainerByHostname(hostname) {
        fmt.Println("✅ Probably a container")
    } else {
        fmt.Println("❌ Doesn't look like a container")
    }

    // 4. PID 1 process check
    fmt.Printf("4. PID 1 process: ")
    if isContainerByInitProcess() {
        fmt.Println("✅ Container")
    } else {
        fmt.Println("❌ Not a container")
    }

    // 5. Docker mounts check
    fmt.Printf("5. Docker mounts: ")
    if hasDockerMounts() {
        fmt.Println("✅ Container")
    } else {
        fmt.Println("❌ Not a container")
    }

    // 6. Architecture and OS
    fmt.Printf("6. Architecture: %s/%s\n", runtime.GOOS, runtime.GOARCH)

    // 7. Environment variables
    fmt.Println("7. Environment variables:")
    for _, env := range os.Environ() {
        if strings.HasPrefix(env, "DOCKER_") || strings.HasPrefix(env, "KUBERNETES_") {
            fmt.Printf("   %s\n", env)
        }
    }
}

// 1. CGroup check (most reliable method)
func isDockerContainerByCGroup() bool {
    file, err := os.ReadFile("/proc/1/cgroup") // Замена ioutil.ReadFile
    if err != nil {
        return false
    }
    
    content := string(file)
    return strings.Contains(content, "docker") || 
           strings.Contains(content, "kubepods") ||
           strings.Contains(content, "containerd")
}

// 2. .dockerenv file check
func isDockerContainerByEnvFile() bool {
    _, err := os.Stat("/.dockerenv")
    return err == nil
}

// 3. Hostname check (less reliable)
func isDockerContainerByHostname(hostname string) bool {
    // Docker containers often have a hash in the name
    return len(hostname) == 64 || 
           strings.Contains(hostname, "docker") ||
           strings.Contains(hostname, "k8s")
}

// 4. Init process check
func isContainerByInitProcess() bool {
    file, err := os.ReadFile("/proc/1/comm") // Замена ioutil.ReadFile
    if err != nil {
        return false
    }
    
    comm := strings.TrimSpace(string(file))
    // In containers, it's often not systemd
    return comm != "systemd" && comm != "init"
}

// 5. Docker mounts check
func hasDockerMounts() bool {
    file, err := os.ReadFile("/proc/mounts") // Замена ioutil.ReadFile
    if err != nil {
        return false
    }
    
    content := string(file)
    return strings.Contains(content, "docker") ||
           strings.Contains(content, "overlay") ||
           strings.Contains(content, "aufs")
}
