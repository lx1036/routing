package main

import (
	database "k8s-lx1036/k8s-ui/backend/database/initial"
	"k8s-lx1036/k8s-ui/backend/initial"
	routers_gin "k8s-lx1036/k8s-ui/backend/routers-gin"
)

const Version = "1.6.1"

func main() {
	/*cmd.Version = Version
	_ = cmd.RootCmd.Execute()*/

	//cmd2.Run()

	database.InitDb()

	// K8S Client
	initial.InitClient()

	// 初始化 rsa key
	initial.InitRsaKey()

	router := routers_gin.SetupRouter()

	_ = router.Run()
}
