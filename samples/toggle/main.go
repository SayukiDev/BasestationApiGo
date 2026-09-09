// スキャンして 1 台目が ON ならスリープに、それ以外なら ON にするサンプル
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
	if len(addrs) == 0 {
		log.Fatal("base station not found")
	}
	api.SetDeviceControl(addrs)
	defer api.Disconnect()

	// 電源状態を取得
	states, err := api.GetPowerState()
	if err != nil {
		log.Println(err)
	}

	// 1 台目が ON なら全部スリープ、それ以外なら全部 ON
	if states[addrs[0]] == "ON" {
		fmt.Println("ON -> sleep")
		err = api.SetPower(api.PwrSleep)
	} else {
		fmt.Println(states[addrs[0]], "-> ON")
		err = api.SetPower(api.PwrBooting)
	}
	if err != nil {
		log.Fatal(err)
	}
}
