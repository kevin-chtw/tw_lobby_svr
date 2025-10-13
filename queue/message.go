package queue

import (
	"fmt"
	"net"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/go-resty/resty/v2"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
)

var postUrl = "https://open.feishu.cn/open-apis/bot/v2/hook/bfbbd819-86ad-4bfa-a5c5-31cda2f0af06"

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		// 检查地址是否是IP地址并且不是环回地址
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func SendCommonMsgQueue(title string, msgList []string) {
	contentParams := [][]map[string]string{
		{
			{
				"tag":  "text",
				"text": fmt.Sprintf("【通知内网ip】: %s", GetLocalIP()),
			},
		},
	}

	for _, text := range msgList {
		contentParams = append(contentParams, []map[string]string{
			{
				"tag":  "text",
				"text": text,
			},
		})
	}

	params := map[string]any{
		"msg_type": "post",
		"content": map[string]any{
			"post": map[string]any{
				"zh_cn": map[string]any{
					"title":   title,
					"content": contentParams,
				},
			},
		},
	}

	{
		str, _ := convertor.ToJson(params)
		logger.Log.Info(`str is:`, str)
	}

	{
		cli := resty.New()
		resp, err := cli.R().SetBody(params).SetHeader("Content-Type", "application/json").Post(postUrl)

		if err != nil {
			logger.Log.Info(`err is:`, err.Error())

		} else {
			logger.Log.Info(`res`, resp)
		}
	}
}
