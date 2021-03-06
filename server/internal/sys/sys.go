/*
   Copyright 2021 Hiroshi.tao

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package sys

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"fairyla/pkg/util"
	"fairyla/vars"
)

type Setting struct {
	Dir       string // html & assets
	Host      string
	Port      uint
	Sapic     Sapic
	Redis     string // storage redis db url
	OpenToken string // send mail api token

	ICP      string // beian.miit.gov.cn
	Beian    string // www.beian.gov.cn
	Slogan   string
	SiteName string
}

type Sapic struct {
	Api   string `json:"api"`
	Field string `json:"field"`
	Token string `json:"token"`
}

var ep = "/api/upload"

func parsePort(sport string) (dport uint, err error) {
	if sport != "" {
		iport, e := strconv.Atoi(sport)
		if e != nil {
			err = e
			return
		}
		dport = uint(iport)
	} else {
		err = errors.New("empty port")
	}
	return
}

func parseApiURL(url string) string {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + url
	}
	if !util.IsValidURL(url) {
		return ""
	}
	url = strings.TrimSuffix(strings.TrimSuffix(url, ep), "/")
	return url + ep
}

// New from cli options first
func New(host string, port uint, redis, api, token, field, dir, opentoken string) *Setting {
	c := &Setting{
		Sapic: Sapic{parseApiURL(api), field, token},
		Redis: redis, Host: host, Port: port, Dir: dir,
		SiteName: vars.DefaultSiteName, OpenToken: opentoken,
	}
	c.parseEnv()
	return c
}

// 从环境变量读取配置，优先级高，会覆盖参数
func (s *Setting) parseEnv() {
	redis := os.Getenv("fairyla_redis_url")
	host := os.Getenv("fairyla_host")
	port := os.Getenv("fairyla_port")
	api := os.Getenv("fairyla_sapic_api")     // upload api url
	field := os.Getenv("fairyla_sapic_field") // upload file field name
	token := os.Getenv("fairyla_sapic_token")

	if redis != "" {
		s.Redis = redis
	}
	if host != "" {
		s.Host = host
	}
	dport, err := parsePort(port)
	if err == nil && dport > 1024 {
		s.Port = dport
	}
	if api != "" {
		s.Sapic.Api = parseApiURL(api)
	}
	if field != "" {
		s.Sapic.Field = field
	}
	if token != "" {
		s.Sapic.Token = token
	}

	// other options
	sep := "fairyla_"
	for _, e := range os.Environ() {
		env := strings.Split(e, "=")
		if len(env) >= 2 && strings.HasPrefix(env[0], sep) {
			field := strings.TrimPrefix(env[0], sep)
			v := env[1]
			switch field {
			case "icp":
				s.ICP = v
			case "beian":
				s.Beian = v
			case "dir":
				s.Dir = v
			case "slogan":
				s.Slogan = v
			case "opentoken":
				s.OpenToken = v
			case "sitename":
				s.SiteName = v
			}
		}
	}
}

func (s *Setting) Pretty() string {
	token := "<No Token>"
	if s.Sapic.Token != "" {
		token = fmt.Sprintf("<%s>", s.Sapic.Token)
	}
	return fmt.Sprintf(`UI @ %s
Host: %s
Port: %d
Redis: %s
Sapic:
    Api: %s
    Field: %s
    LinkToken: %s
OpenToken: %s`,
		s.Dir, s.Host, s.Port, s.Redis, s.Sapic.Api, s.Sapic.Field, token,
		s.OpenToken,
	)
}

// 检查是否缺少必须项
func (s *Setting) Check() error {
	sa := s.Sapic
	err := ""
	if s.Redis == "" {
		err = "redis"
	} else if sa.Api == "" {
		err = "sapic-url"
	} else if sa.Token == "" {
		err = "sapic-token"
	}
	if err != "" {
		return fmt.Errorf("miss required option: %s", err)
	}
	return nil
}

// 站点公共配置项
func (s *Setting) SitePublic() map[string]interface{} {
	cfg := make(map[string]interface{})
	cfg["icp"] = s.ICP
	cfg["beian"] = s.Beian
	cfg["slogan"] = s.Slogan
	cfg["upload_limit"] = vars.UploadLimitSize
	cfg["site_name"] = s.SiteName
	return cfg
}
