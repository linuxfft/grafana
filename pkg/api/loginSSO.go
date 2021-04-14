package api

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/grafana/grafana/pkg/setting"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gopkg.in/resty.v1"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/login"
	"github.com/grafana/grafana/pkg/models"
)

var (
	ssoLogger        = log.New("sso")
	ENV_SUC_ROOT_URL = os.Getenv("ENV_SUC_ROOT_URL")

	glbClient = httpClient{}
)

type httpClient struct {
	httpClient *resty.Client
}

func newHttpClient() *resty.Client {
	return resty.New().SetRetryCount(3).
		SetHeader("Content-Type", "application/json").
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(3 * time.Second).
		SetTimeout(15 * time.Second)
}

func (c *httpClient) ensureHttpClient() *resty.Client {
	if c.httpClient != nil {
		return c.httpClient
	}
	client := newHttpClient()
	c.httpClient = client
	return client
}

func (c *httpClient) doSSOLoginAuth(crcCode string) (dtos.SUCLoginResp, error) {
	client := c.ensureHttpClient()

	//payload := sucLoginAuthReq{Domain: "local", Account: username, Password: password}
	payload := map[string]string{
		"code": crcCode,
	}
	var r dtos.SUCLoginResp
	url := fmt.Sprintf("%s/%s", ENV_SUC_ROOT_URL, "accounts/crcCodeLogin")
	resp, err := client.R().SetQueryParams(payload).Get(url)
	if err != nil {
		ssoLogger.Error("Post Auth Login Error: %s", err.Error())
		return r, err
	}
	body := resp.Body()
	ssoLogger.Debug(fmt.Sprintf("Do Auth Login, Resp: Status Code %d, Data: %s", resp.StatusCode(), string(body)))
	if err := json.Unmarshal(body, &r); err != nil {
		return r, err
	}
	if !r.Success {
		return r, errors.Errorf("Auth Fail, Message: %s, Data: %v", r.Message, r.Data)
	}
	return r, nil
}

func mockResp(cc string) (dtos.SUCLoginResp, error) {
	return dtos.SUCLoginResp{
		Message: "ok",
		Data: dtos.SUCLoginRespData{
			Account: "123124214142",
			Domain:  "222",
		},
		Success: true,
	}, nil
}

func ssoBuildExternalUserInfo(resp *dtos.SUCLoginResp) *models.ExternalUserInfo {
	uuid, _ := uuid.NewUUID()
	extUser := &models.ExternalUserInfo{
		AuthModule: "sso",
		AuthId:     uuid.String(),
	}
	extUser.Login = resp.Data.Account
	extUser.Email = resp.Data.Account

	return extUser

}

func ssoSyncUser(
	ctx *models.ReqContext,
	extUser *models.ExternalUserInfo,
) (*models.User, error) {
	ssoLogger.Debug("Syncing Grafana user with corresponding SSO profile")
	// add/update user in Grafana
	cmd := &models.UpsertUserCommand{
		ReqContext:    ctx,
		ExternalUser:  extUser,
		SignupAllowed: true,
	}
	if err := bus.Dispatch(cmd); err != nil {
		return nil, err
	}

	// Do not expose disabled status,
	// just show incorrect user credentials error (see #17947)
	if cmd.Result.IsDisabled {
		oauthLogger.Warn("User is disabled", "user", cmd.Result.Login)
		return nil, login.ErrInvalidCredentials
	}

	return cmd.Result, nil
}

func (hs *HTTPServer) LoginSSOView(ctx *models.ReqContext) {
	var user *models.User

	crccode := ctx.Req.URL.Query().Get("crccode")

	ssoLogger.Info("SSO Login With CRC", crccode)

	if crccode == "" {
		ssoLogger.Error("Login SSO CRCCode Is Empty")
		return
	}

	resp, err := glbClient.doSSOLoginAuth(crccode)

	//resp, err:= mockResp(crccode)

	if err != nil {
		ssoLogger.Error(fmt.Sprintf("doSSOLoginAuth Error: %s", err.Error()), ctx)
		return
	}

	extUserInfo := ssoBuildExternalUserInfo(&resp)
	user, err = ssoSyncUser(ctx, extUserInfo)
	if err != nil {
		ssoLogger.Error(fmt.Sprintf("Sync User Error: %s", err.Error()), ctx)
		return
	}

	if err := hs.loginUserWithUser(user, ctx); err != nil {
		ssoLogger.Error(fmt.Sprintf("loginUserWithUser User Error: %s", err.Error()), ctx)
		return
	}

	ctx.Redirect(setting.AppSubUrl + "/")

}
