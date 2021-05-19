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

package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"fairyla/internal/album"
	"fairyla/internal/user/auth"
	"fairyla/vars"

	"github.com/labstack/echo/v4"
	"tcw.im/gtc"
)

func customHTTPErrorHandler(err error, c echo.Context) {
	code := 400
	msg := err.Error()
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		msg = he.Message.(string)
	}
	if msg == vars.RedigoNil {
		msg = "no data"
	}
	c.JSON(code, vars.ResErr(getLocale(c), msg))
}

// 注册
func signUpView(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")
	err := auth.Register(rc, username, password)
	if err != nil {
		return err
	}
	return c.JSON(200, vars.ResOK())
}

// 登录
func signInView(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")
	remember := gtc.IsTrue(c.FormValue("remember"))
	expire := 60 * 60 * 2 // 2h
	if remember {
		expire = 60 * 60 * 24 * 7 // 7d
	}
	token, err := auth.Login(rc, username, password, remember)
	if err != nil {
		return err
	}
	data := make(map[string]interface{})
	data["token"] = token
	data["expire"] = expire
	return c.JSON(200, vars.NewResData(data))
}

// 前端公共配置、用户状态等
func configView(c echo.Context) error {
	data := cfg.SitePublic()
	data["isLogin"] = false
	user, err := checkJWT(c)
	if err == nil {
		data["isLogin"] = true
	}
	data["user"] = user
	return c.JSON(200, vars.NewResData(data))
}

// 列出所有公共专辑或获取某个公共专辑属性及其下照片
func pubAlbumView(c echo.Context) error {
	w := album.New(rc)
	if gtc.IsTrue(c.QueryParam("fairy")) {
		user := c.QueryParam("user")
		albumID := getAlbumID(user, c.QueryParam("album"))
		if user == "" || albumID == "" {
			return errors.New("invalid param")
		}
		if user != album.AlbumID2User(albumID) {
			return errors.New("album id does not match user")
		}
		data, err := w.GetAlbumFairies(user, albumID)
		if err != nil {
			return err
		}
		return c.JSON(200, vars.NewResData(data))
	} else {
		data, err := w.ListPublicAlbums()
		if err != nil {
			return err
		}
		return c.JSON(200, vars.NewResData(data))
	}
}

// 上传图片、视频
func uploadView(c echo.Context) error {
	file, err := c.FormFile("file")
	if err != nil {
		return err
	}
	// Size(bytes) limit
	limit := vars.UploadLimitSize * 1024 * 1024
	if file.Size > limit {
		return errors.New("the uploaded file exceeds the limit")
	}
	fd, err := file.Open()
	if err != nil {
		return err
	}
	defer fd.Close()

	content, err := ioutil.ReadAll(fd)
	if err != nil {
		return err
	}
	stream := base64.StdEncoding.EncodeToString(content)

	var post http.Request
	post.ParseForm()
	post.Form.Add(cfg.Sapic.Field, stream)
	post.Form.Add("album", "fairyla")
	post.Form.Add("filename", file.Filename)
	post.Form.Add("_upload_field", cfg.Sapic.Field)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(
		"POST", cfg.Sapic.Api, strings.NewReader(post.Form.Encode()),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", echo.MIMEApplicationForm)
	req.Header.Set("User-Agent", "fairyla/v1")
	req.Header.Set("Authorization", "LinkToken "+cfg.Sapic.Token)
	req.Header.Set("Accept-Language", c.Request().Header.Get("Accept-Language"))

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	ret := struct {
		Code int    `json:"-"`
		Msg  string `json:"msg"`
		Src  string `json:"src"`
	}{}
	err = json.Unmarshal(body, &ret)
	if err != nil {
		return err
	}
	if ret.Src != "" {
		d := map[string]string{"src": ret.Src}
		return c.JSON(200, vars.NewResData(d))
	} else {
		return errors.New(ret.Msg)
	}
}

/*
 * AlbumView 专辑视图
 *
 * 状态：已登录
 *
 * - create、drop、update、get、list：增、删、改、查（单）、查（多）
 */

// 创建专辑
func createAlbumView(c echo.Context) error {
	name := c.FormValue("name")
	labels := c.FormValue("labels")
	a, err := album.NewAlbum(getUser(c), name)
	if err != nil {
		return err
	}
	has, err := a.Exist(rc)
	if err != nil {
		return err
	}
	if has {
		return errors.New("album already exists")
	}
	if labels != "" {
		for _, label := range strings.Split(labels, ",") {
			a.AddLabel(strings.TrimSpace(label))
		}
	}
	w := album.New(rc)
	err = w.WriteAlbum(a)
	if err != nil {
		return err
	}
	return c.JSON(200, vars.NewResData(a))
}

// 更新专辑属性
func updateAlbumView(c echo.Context) error {
	user := getUser(c)
	albumID := autoAlbumID(c)
	w := album.New(rc)
	a, err := w.GetAlbum(user, albumID)
	if err != nil {
		return err
	}
	hookWriteShare := false
	switch getParam(c, "action") {
	case "status":
		pub := c.FormValue("public")
		if pub == "" {
			a.Public = !a.Public
		} else {
			a.Public = gtc.IsTrue(pub)
		}
	case "share":
		ta := c.FormValue("ta")
		has, err := w.HasUser(ta)
		if err != nil {
			return err
		}
		if !has {
			return errors.New("not found username")
		}
		a.Ta = ta
		hookWriteShare = true
	default:
		return errors.New("invalid action param")
	}
	err = w.WriteAlbum(&a)
	if err != nil {
		return err
	}
	if hookWriteShare {
		err = (&a).Claim(rc)
		if err != nil {
			return err
		}
	}
	return c.JSON(200, vars.ResOK())
}

// 删除专辑
func dropAlbumView(c echo.Context) error {
	user := getUser(c)
	albumID := autoAlbumID(c)
	w := album.New(rc)
	err := w.DropAlbum(user, albumID)
	if err != nil {
		return err
	}
	return c.JSON(200, vars.ResOK())
}

// 获取用户所有专辑信息
func listAlbumView(c echo.Context) error {
	w := album.New(rc)
	data, err := w.ListAlbums(getUser(c))
	if err != nil {
		return err
	}
	return c.JSON(200, vars.NewResData(data))
}

// 获取用户所有专辑名（包含认领）
func listAlbumNamesView(c echo.Context) error {
	w := album.New(rc)
	user := getUser(c)
	names := []string{}
	albums, err := w.ListAlbums(user)
	if err != nil {
		return err
	}
	for _, a := range albums {
		names = append(names, a.Name)
	}
	claims, err := w.ListClaimAlbums(user)
	if err != nil {
		return err
	}
	for _, a := range claims {
		names = append(names, fmt.Sprintf("%s/%s", a.Owner, a.Name))
	}
	return c.JSON(200, vars.NewResData(names))
}

// 获取用户某个专辑信息
func getAlbumView(c echo.Context) error {
	w := album.New(rc)
	user := getUser(c)
	albumID := autoAlbumID(c)
	if gtc.IsTrue(c.QueryParam("fairy")) {
		data, err := w.GetAlbumFairies(user, albumID)
		if err != nil {
			return err
		}
		return c.JSON(200, vars.NewResData(data))
	} else {
		data, err := w.GetAlbum(user, albumID)
		if err != nil {
			return err
		}
		return c.JSON(200, vars.NewResData(data))
	}
}

// 仅获取专辑下所有照片信息
func listFariesOfAlbumView(c echo.Context) error {
	if !eqUserAlbumID(c) {
		return errors.New("not found album")
	}
	w := album.New(rc)
	data, err := w.GetFairies(autoAlbumID(c))
	if err != nil {
		return err
	}
	return c.JSON(200, vars.NewResData(data))
}

/*
 * FairyView 照片视图
 *
 * 状态：已登录
 */

// 创建照片
func createFairyView(c echo.Context) error {
	w := album.New(rc)
	user := getUser(c)
	albumID := c.FormValue("album_id")
	albumName := c.FormValue("album_name")
	src := c.FormValue("src")
	desc := c.FormValue("desc")

	// 一定解析出专辑属主和ID，并判断是否为认领。
	// 如果为认领，属主和ID都是专辑原有的；否则是当前用户。
	isClaim := false
	albumOwner := user
	var a *album.Album
	if albumID == "" {
		// name required
		if albumName == "" {
			return errors.New("invalid album_id or album_name")
		}
		if strings.Contains(albumName, "/") {
			isClaim = true
			an := strings.Split(albumName, "/")
			albumOwner = an[0]
			albumName = an[1]
		}
		albumID = album.AlbumName2ID(albumOwner, albumName)
		a, _ = album.NewAlbum(albumOwner, albumName)
		exist, err := a.Exist(rc)
		if err != nil {
			return err
		}
		if exist {
			da, err := w.GetAlbum(albumOwner, albumID)
			if err != nil {
				return err
			}
			a = &da
		} else {
			if isClaim {
				log.Println("claim album is not exists")
				return errors.New("not found album")
			}
			// 不存在时，即新建专辑
		}
	} else {
		if strings.Contains(albumID, "/") {
			isClaim = true
			an := strings.Split(albumID, "/")
			if an[0] != album.AlbumID2User(an[1]) {
				return errors.New("album id does not match user")
			}
			albumOwner = an[0]
			albumID = an[1]
		} else {
			// 拒绝不是用户名下的专辑ID
			if user != album.AlbumID2User(albumID) {
				log.Println("album(id) does not belong to this user")
				return errors.New("not found album")
			}
		}
		// 专辑必须存在
		da, err := w.GetAlbum(albumOwner, albumID)
		if err != nil {
			return err
		}
		a = &da
	}
	if isClaim {
		has, err := w.HasClaim(user, albumID)
		if err != nil {
			return err
		}
		if !has {
			log.Println("upload to a claim that does not belong to the user")
			return errors.New("not found claim")
		}
	}

	f, err := album.NewFairy(user, albumID, src, desc)
	if err != nil {
		return err
	}
	if !f.IsVideo {
		a.SetLatest(f)
	}
	err = w.WriteAlbum(a)
	if err != nil {
		return err
	}
	err = w.WriteFairy(f)
	if err != nil {
		return err
	}
	return c.JSON(200, vars.NewResData(f))
}

// 删除照片
func dropFairyView(c echo.Context) error {
	if !eqUserAlbumID(c) {
		return errors.New("not found album")
	}
	albumID := autoAlbumID(c)
	fairyID := c.FormValue("fairy_id")
	if fairyID == "" {
		return errors.New("invalid param")
	}
	w := album.New(rc)
	err := w.DropFairy(albumID, fairyID)
	if err != nil {
		return err
	}
	return c.JSON(200, vars.ResOK())
}

// 获取用户所有照片信息
func listFairyView(c echo.Context) error {
	w := album.New(rc)
	data, err := w.ListFairies(getUser(c))
	if err != nil {
		return err
	}
	return c.JSON(200, vars.NewResData(data))
}

/*
 * ClaimView 认领专辑视图
 *
 * 状态：已登录
 */

// 获取用户认领专辑信息
func listClaimView(c echo.Context) error {
	w := album.New(rc)
	if gtc.IsTrue(c.QueryParam("fairy")) {
		user := getUser(c)
		owner := c.QueryParam("owner")
		albumID := getAlbumID(owner, c.QueryParam("album"))
		if owner == "" || albumID == "" {
			return errors.New("invalid param")
		}
		if owner != album.AlbumID2User(albumID) {
			return errors.New("album id does not match user")
		}
		has, err := w.HasClaim(user, albumID)
		if err != nil {
			return err
		}
		if !has {
			return errors.New("not found claim")
		}
		data, err := w.GetAlbumFairies(owner, albumID)
		if err != nil {
			return err
		}
		return c.JSON(200, vars.NewResData(data))
	} else {
		data, err := w.ListClaimAlbums(getUser(c))
		if err != nil {
			return err
		}
		return c.JSON(200, vars.NewResData(data))
	}
}

// 认领其他用户专辑（需由所属者确认方可领取成功）
func createClaimView(c echo.Context) error {
	return nil
}
