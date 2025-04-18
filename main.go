package main

import (
    "fmt"
    "io"
    "log"
    "net"
    "os"

    "golang.org/x/crypto/ssh"
    "gopkg.in/yaml.v2"
)

type Config struct {
    SSH struct {
        Host     string `yaml:"host"`
        Port     int    `yaml:"port"`
        User     string `yaml:"user"`
        Password string `yaml:"password"`
    } `yaml:"ssh"`
    Forwards []struct {
        Local  string `yaml:"local"`
        Remote string `yaml:"remote"`
    } `yaml:"forwards"`
}

func loadConfig(path string) (*Config, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    var cfg Config
    decoder := yaml.NewDecoder(f)
    err = decoder.Decode(&cfg)
    return &cfg, err
}

func startForward(client *ssh.Client, localAddr, remoteAddr string) {
    listener, err := net.Listen("tcp", localAddr)
    if err != nil {
        log.Fatalf("无法监听本地地址 %s: %v", localAddr, err)
    }
    log.Printf("端口转发: %s -> %s", localAddr, remoteAddr)
    go func() {
        for {
            localConn, err := listener.Accept()
            if err != nil {
                log.Printf("接受本地连接失败: %v", err)
                continue
            }

            go func() {
                remoteConn, err := client.Dial("tcp", remoteAddr)
                if err != nil {
                    log.Printf("连接远程地址失败 %s: %v", remoteAddr, err)
                    localConn.Close()
                    return
                }

                // 双向拷贝
                go io.Copy(remoteConn, localConn)
                go io.Copy(localConn, remoteConn)
            }()
        }
    }()
}

func main() {
    if len(os.Args) < 3 || os.Args[1] != "-config" {
        fmt.Println("用法: ssh-forward -config config.yaml")
        os.Exit(1)
    }

    configPath := os.Args[2]
    cfg, err := loadConfig(configPath)
    if err != nil {
        log.Fatalf("配置加载失败: %v", err)
    }

    // SSH 连接配置
    sshConfig := &ssh.ClientConfig{
        User: cfg.SSH.User,
        Auth: []ssh.AuthMethod{
            ssh.Password(cfg.SSH.Password),
        },
        HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 测试用，建议改成可信主机验证
    }

    addr := fmt.Sprintf("%s:%d", cfg.SSH.Host, cfg.SSH.Port)
    client, err := ssh.Dial("tcp", addr, sshConfig)
    if err != nil {
        log.Fatalf("SSH连接失败: %v", err)
    }

    for _, f := range cfg.Forwards {
        startForward(client, f.Local, f.Remote)
    }

    // 保持程序运行
    select {}
}
