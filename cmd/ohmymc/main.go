package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	assets "ohmymc"
	"ohmymc/internal/app"
	"ohmymc/internal/desktop"
	"os"
	"os/signal"
	"time"
)

func main() {
	root := flag.String("data", "", "启动器配置目录，默认用户配置目录/OhMyMC")
	games := flag.String("games", "", "游戏安装目录，默认启动器所在目录/.minecraft")
	serve := flag.Bool("serve", false, "在回环地址运行 WebUI 开发服务（无原生窗口）")
	addr := flag.String("addr", "127.0.0.1:17843", "开发服务回环监听地址")
	flag.Parse()
	paths, err := resolvePaths(*root, *games)
	if err != nil {
		log.Fatal(err)
	}
	service, err := app.NewWithMinecraftRoot(paths.Data, paths.Games)
	if err != nil {
		log.Fatal(err)
	}
	defer service.Close()
	if !*serve {
		if err = desktop.Run(service, assets.Frontend()); err != nil {
			log.Print(err)
		}
		return
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		log.Fatal(err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		log.Fatal("开发服务只能绑定回环 IP 地址")
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Handler: app.Handler(service, assets.Frontend(), ln.Addr().String()), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	fmt.Printf("OhMyMC WebUI: http://%s\nData: %s\nGames: %s\n", ln.Addr(), service.Store.Root(), service.Store.MinecraftRoot())
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt)
	defer signal.Stop(done)
	go func() {
		<-done
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	if err = server.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Print(err)
	}
}
