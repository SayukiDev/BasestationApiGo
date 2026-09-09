// スキャンして電源状態を表示するサンプル
package main

import (
	"fmt"
	"log"
	"time"

	api "github.com/SayukiDev/BasesationApiGo"
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

	// 電源状態を取得
	states, err := api.GetPowerState()
	if err != nil {
		log.Println(err)
	}
	for addr, state := range states {
		fmt.Println(addr, state)
	}
}
