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
	"net/url"
	"strings"
	"time"

	"fairyla/internal/album"
	uif "fairyla/internal/user"
	"fairyla/internal/user/auth"
	"fairyla/internal/user/event"
	"fairyla/pkg/util"
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
	c.JSON(code, vars.ResErrLocale(getLocale(c), msg))
}

func readyView(c echo.Context) error {
	ping, err := rc.Ping()
	if err != nil || !ping {
		return c.JSONBlob(503, []byte(`"err"`))
	}
	return c.JSONBlob(200, []byte(`"ok"`))
}

// 注册
func signUpView(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")
	email := c.FormValue("email")
	p := uif.NewProfile(username)
	s := uif.NewSetting()
	p.Email = email
	err := auth.Register(rc, username, password, p, s)
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

// 忘记密码
func forgotView(c echo.Context) error {
	username := c.FormValue("username")
	w := uif.New(rc)
	if has, err := w.HasUser(username); !has {
		return err
	}
	p, err := w.UserProfile(username)
	if err != nil {
		return err
	}
	if p.Email == "" {
		return errors.New("invalid email")
	}
	// 生成邮箱验证码
	token, err := auth.GenerateForgotJWT(rc, username)
	if err != nil {
		return err
	}
	// 发送邮箱验证码
	siteURL := fmt.Sprintf("%s://%s", c.Scheme(), c.Request().Host)
	resetURI := "/reset_passwd"
	verifyURL := fmt.Sprintf("%s/#%s?jwt=%s", siteURL, resetURI, token)
	tpl, err := vars.NewForgot(siteURL, cfg.SiteName, username, verifyURL)
	if err != nil {
		return err
	}
	resp, err := http.PostForm(
		"https://open.saintic.com/api/sendmail",
		url.Values{
			"token":     {cfg.OpenToken},
			"from_name": {cfg.SiteName},
			"to":        {p.Email},
			"subject":   {fmt.Sprintf("重置密码 | %s", cfg.SiteName)},
			"html":      {tpl},
		},
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	ret := struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}{}
	err = json.Unmarshal(body, &ret)
	if err != nil {
		return err
	}
	if ret.Code == 0 && ret.Msg == "" {
		return c.JSON(200, vars.ResOK())
	}
	return errors.New(ret.Msg)
}

// 重置密码（依附于忘记密码，不接受普通token认证）
func resetPasswdView(c echo.Context) error {
	// 由 loginRequired 验证令牌，通过进入此视图，即可直接重置
	token := c.FormValue("jwt")
	passwd := c.FormValue("password")
	user, err := checkJWT(token)
	if err != nil {
		return err
	}
	err = auth.ResetPasswd(rc, user, passwd)
	if err != nil {
		return err
	}
	return c.JSON(200, vars.ResOK())
}

// 前端公共配置、用户状态等
func configView(c echo.Context) error {
	data := cfg.SitePublic()
	data["isLogin"] = false
	user, err := autoCheckJWT(c)
	if err == nil {
		data["isLogin"] = true
		userinfo, err := uif.New(rc).UserData(user)
		if err == nil {
			data["userinfo"] = userinfo
		}
	}
	data["user"] = user
	return c.JSON(200, vars.NewResData(data))
}

// 列出所有公共专辑
func listPublicAlbumView(c echo.Context) error {
	w := album.New(rc)
	data, err := w.ListPublicAlbums()
	if err != nil {
		return err
	}
	return c.JSON(200, vars.NewResData(data))
}

// 获取某个公共专辑信息及其下照片
func getPublicAlbumView(c echo.Context) error {
	w := album.New(rc)
	owner := c.Param("owner")
	albumID := getAlbumID(owner, c.Param("id"))
	withFairy := gtc.IsTrue(c.QueryParam("fairy"))
	if owner == "" || albumID == "" {
		return errors.New("invalid param")
	}
	if owner != album.AlbumID2User(albumID) {
		return errors.New("album id does not match user")
	}
	is404 := false
	var ret interface{}
	if withFairy {
		af, err := w.GetAlbumFairies(owner, albumID)
		if err != nil {
			return err
		}
		if af.Public {
			ret = af
		} else {
			is404 = true
		}
	} else {
		a, err := w.GetAlbum(owner, albumID)
		if err != nil {
			return err
		}
		if a.Public {
			ret = a
		} else {
			is404 = true
		}
	}
	if is404 {
		return c.JSON(404, vars.ResErr("not found"))
	} else {
		return c.JSON(200, vars.NewResData(ret))
	}
}

/*
 * UserView 用户视图
 *
 * 状态：已登录
 *
 * - create、drop、update、get、list：增、删、改、查（单）、查（多）
 */

func getUserView(c echo.Context) error {
	w := uif.New(rc)
	user := getUser(c)
	data, err := w.UserData(user)
	if err != nil {
		return err
	}
	return c.JSON(200, vars.NewResData(data))
}

// 更新用户资料
func updateUserProfileView(c echo.Context) error {
	alias := c.FormValue("alias")
	bio := c.FormValue("bio")
	email := c.FormValue("email")
	user := getUser(c)
	w := uif.New(rc)
	p, err := w.UserProfile(user)
	if err != nil {
		return err
	}
	if alias != "" {
		p.Alias = alias
	}
	if bio != "" {
		p.Bio = bio
	}
	if email != "" && util.IsEmail(email) {
		p.Email = email
	}
	err = w.UpdateProfile(p)
	if err != nil {
		return err
	}
	return c.JSON(200, vars.NewResData(p))
}

// 更改密码
func updateUserPasswdView(c echo.Context) error {
	opwd := c.FormValue("old_passwd")
	npwd := c.FormValue("new_passwd")
	user := getUser(c)
	if ok, err := auth.VerifyPasswd(rc, user, opwd); !ok {
		return err
	}
	if err := auth.ResetPasswd(rc, user, npwd); err != nil {
		return err
	}
	return c.JSON(200, vars.ResOK())
}

// 更新用户设置
func updateUserSettingView(c echo.Context) error {
	adp := c.FormValue("album_default_public")
	slogan := c.FormValue("slogan")
	user := getUser(c)
	w := uif.New(rc)
	s, err := w.UserSetting(user)
	if err != nil {
		return err
	}
	if adp != "" {
		s.AlbumDefaultPublic = gtc.IsTrue(adp)
	}
	if slogan != "" {
		s.Slogan = slogan
		if gtc.IsFalse(slogan) { // 取消
			s.Slogan = ""
		}
	}
	err = w.UpdateSetting(user, s)
	if err != nil {
		return err
	}
	return c.JSON(200, vars.NewResData(s))
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

	client := &http.Client{Timeout: 60 * time.Second}
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
 */

// 创建专辑
func createAlbumView(c echo.Context) error {
	name := c.FormValue("name")
	labels := c.FormValue("labels")
	a, err := album.NewAlbum(getUser(c), name)
	if err != nil {
		return err
	}
	a.FillStatus(rc)
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
	action := getParam(c, "action")
	hookWriteShare := false
	switch action {
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
		if ta == user {
			return errors.New("cannot share to yourself")
		}
		if a.Ta != "" && a.Ta == ta {
			return errors.New("already belong ta")
		}
		// 先删除当前
		if a.Ta != "" {
			err = (&a).Claim(rc, true)
			if err != nil {
				return err
			}
		}
		a.Ta = ta
		hookWriteShare = true
		// 入库前，清空认领申请列表
		a.Opt.ClaimingBy = []string{}
	case "label", "add_label", "remove_label":
		label := c.FormValue("label")
		label = strings.TrimSuffix(strings.TrimSpace(label), ",")
		if label == "" {
			return errors.New("invalid param")
		}
		labels := strings.Split(label, ",")
		if action == "add_label" || action == "remove_label" {
			for _, l := range labels {
				if action == "add_label" {
					a.AddLabel(l)
				} else {
					a.RemoveLabel(l)
				}
			}
		} else {
			a.Label = labels
		}
	default:
		return errors.New("invalid action param")
	}
	err = w.WriteAlbum(&a)
	if err != nil {
		return err
	}
	// 专辑数据入库后的后续钩子处理
	if hookWriteShare {
		err = (&a).Claim(rc, false)
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
	withFairy := gtc.IsTrue(c.QueryParam("fairy"))
	var ret interface{}
	if withFairy {
		data, err := w.GetAlbumFairies(user, albumID)
		if err != nil {
			return err
		}
		ret = data
	} else {
		data, err := w.GetAlbum(user, albumID)
		if err != nil {
			return err
		}
		ret = data
	}
	return c.JSON(200, vars.NewResData(ret))
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
		a.FillStatus(rc)
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

/*
 * ClaimView 认领专辑视图
 *
 * 状态：已登录
 */

// 认领其他用户专辑（需由所属者确认方可领取成功）
func createClaimView(c echo.Context) error {
	user := getUser(c)
	owner := c.Param("owner")
	id := c.Param("id")
	if owner == "" || id == "" {
		return errors.New("invalid param")
	}
	w := album.New(rc)
	err := w.CreateClaim(user, owner, getAlbumID(owner, id))
	if err != nil {
		return err
	}
	return c.JSON(200, vars.ResOK())
}

// 获取用户认领的专辑数据及其下照片
func getClaimView(c echo.Context) error {
	w := album.New(rc)
	user := getUser(c)
	owner := c.Param("owner")
	albumID := getAlbumID(owner, c.Param("id"))
	withFairy := gtc.IsTrue(c.QueryParam("fairy"))
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
	var ret interface{}
	if withFairy {
		af, err := w.GetAlbumFairies(owner, albumID)
		if err != nil {
			return err
		}
		ret = af
	} else {
		a, err := w.GetAlbum(owner, albumID)
		if err != nil {
			return err
		}
		ret = a
	}
	return c.JSON(200, vars.NewResData(ret))
}

// 获取用户认领专辑列表
func listClaimView(c echo.Context) error {
	w := album.New(rc)
	data, err := w.ListClaimAlbums(getUser(c))
	if err != nil {
		return err
	}
	return c.JSON(200, vars.NewResData(data))
}

// 删除认领的用户专辑
func dropClaimView(c echo.Context) error {
	user := getUser(c)
	owner := c.Param("owner")
	albumID := getAlbumID(owner, c.Param("id"))
	if owner == "" || albumID == "" {
		return errors.New("invalid param")
	}
	w := album.New(rc)
	if has, _ := w.HasClaim(user, albumID); !has {
		return errors.New("not found claim")
	}
	err := w.DropClaim(user, owner, albumID)
	if err != nil {
		return err
	}
	return c.JSON(200, vars.ResOK())
}

/*
 * EventView 用户事件视图
 *
 * 状态：已登录
 */

// 创建事件
func createEventView(c echo.Context) error {
	user := getUser(c)
	action := getParam(c, "action")
	if action == "message" {
		msg := c.FormValue("message")
		class := c.FormValue("classify")
		m, err := event.NewMessage(user, msg, class)
		if err != nil {
			return err
		}
		err = m.Write(rc)
		if err != nil {
			return err
		}
		return c.JSON(200, vars.ResOK())
	}
	return errors.New("invalid action param")
}

// 查询事件
func listEventView(c echo.Context) error {
	w := event.New(rc)
	ms, err := w.ListMessages(getUser(c), c.FormValue("classify"))
	if err != nil {
		return err
	}
	Accept := c.Request().Header.Get("Accept")
	if strings.Contains(Accept, "application/json") {
		return c.JSON(200, vars.NewResData(ms))
	}
	ret, err := json.Marshal(ms)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	res := fmt.Sprintf("retry: 30000\ndata: %s\n\n", string(ret))
	return c.String(200, res)
}

// 删除事件
func dropEventView(c echo.Context) error {
	w := event.New(rc)
	err := w.DropMessages(getUser(c), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(200, vars.ResOK())
}
