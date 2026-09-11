// スキャンせずにアドレス指定で接続して電源状態を取得するサンプル
package main

import (
	"fmt"
	"log"

	api "github.com/SayukiDev/BasestationApiGo"
)

func main() {
	// スキャンしていなければ、アドレス文字列がそのまま接続先として使われる
	api.SetDeviceControl([]string{"E5:2A:28:10:58:89"})
	defer api.Disconnect()

	states, err := api.GetPowerState()
	if err != nil {
		log.Fatal(err)
	}
	for addr, state := range states {
		fmt.Println(addr, state)
	}
}
