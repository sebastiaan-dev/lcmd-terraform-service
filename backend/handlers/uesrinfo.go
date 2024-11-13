package handlers

import (
	"os"

	gohelper "gitee.com/linakesi/lzc-sdk/lang/go"
	users "gitee.com/linakesi/lzc-sdk/lang/go/common"

	"github.com/gin-gonic/gin"
)

func listAllUers() []string {
	var ret []string
	// 纯前端无法获取其他用户信息，但后端lzcapp是可以获取所有用户信息的
	// 不论是单实例还是多实例，如果管理员允许lzcapp获取用户文稿，则会将
	// 对应文稿目录按照uid的方式放在这里。
	// 然后结合sdk里的Users.QueryUserRole rpc则可以获取所有的用户信息
	dirs, err := os.ReadDir("/lzcapp/run/mnt/home")
	if err != nil {
		return nil
	}
	for _, d := range dirs {
		ret = append(ret, d.Name())
	}
	return ret
}

type LoginInfo struct {
	DeviceID      string
	DeviceVersion string
	UserId        string
	UserRole      string
}

func GetUserInfo(c *gin.Context) {
	ctx := c.Request.Context()
	gw, err := gohelper.NewAPIGateway(ctx)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	var ret struct {
		CurrentUserInfo LoginInfo
		AllUserInfos    []*users.UserInfo
	}
	ret.CurrentUserInfo = LoginInfo{
		UserId:        c.GetHeader("x-hc-user-id"),
		UserRole:      c.GetHeader("x-hc-user-role"),
		DeviceID:      c.GetHeader("x-hc-device-id"),
		DeviceVersion: c.GetHeader("x-hc-device-version"),
	}
	for _, uid := range listAllUers() {
		uinfo, err := gw.Users.QueryUserInfo(ctx, &users.UserID{Uid: uid})
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		ret.AllUserInfos = append(ret.AllUserInfos, uinfo)
	}
	c.JSON(200, ret)
}
