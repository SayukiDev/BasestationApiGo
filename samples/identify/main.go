// スキャンして全部識別（LED 点滅）するサンプル
package main

import (
	"fmt"
	"log"
	"time"

	api "github.com/SayukiDev/BasestationApiGo"
)

func main() {
	// 5 秒間スキャン
	if err := api.ScanningWithTimeout(5 * time.Second); err != nil {
		log.Fatal(err)
	}

	// 見つかったベースステーションを全部制御対象にする
	var addrs []string
	for _, s := range api.GetBaseStation() {
		fmt.Println("found:", s.Name, s.Addr)
		addrs = append(addrs, s.Addr)
	}
	api.SetDeviceControl(addrs)
	defer api.Disconnect()

	// 1 台ずつ識別
	for _, addr := range addrs {
		if err := api.Identify(addr); err != nil {
			log.Println(addr, err)
			continue
		}
		fmt.Println(addr, "identify OK")
	}
}
