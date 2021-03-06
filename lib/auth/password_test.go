/*
Copyright 2017-2018 Gravitational, Inc.

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

package auth

import (
	"context"
	"encoding/base32"
	"fmt"
	"time"

	"github.com/gravitational/teleport"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	. "gopkg.in/check.v1"
)

type PasswordSuite struct {
	bk             backend.Backend
	a              *AuthServer
	mockedAuditLog *events.MockAuditLog
}

var _ = fmt.Printf
var _ = Suite(&PasswordSuite{})

func (s *PasswordSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *PasswordSuite) TearDownSuite(c *C) {
}

func (s *PasswordSuite) SetUpTest(c *C) {
	var err error
	c.Assert(err, IsNil)
	s.bk, err = lite.New(context.TODO(), backend.Params{"path": c.MkDir()})
	c.Assert(err, IsNil)

	// set cluster name
	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	c.Assert(err, IsNil)
	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                s.bk,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	s.a, err = NewAuthServer(authConfig)
	c.Assert(err, IsNil)

	err = s.a.SetClusterName(clusterName)
	c.Assert(err, IsNil)

	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		LocalAuth: services.NewBool(true),
	})
	c.Assert(err, IsNil)

	err = s.a.SetClusterConfig(clusterConfig)
	c.Assert(err, IsNil)

	// set static tokens
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionTokenV1{},
	})
	c.Assert(err, IsNil)
	err = s.a.SetStaticTokens(staticTokens)
	c.Assert(err, IsNil)

	s.mockedAuditLog = events.NewMockAuditLog(0)
	s.a.IAuditLog = s.mockedAuditLog
}

func (s *PasswordSuite) TearDownTest(c *C) {
}

func (s *PasswordSuite) TestTiming(c *C) {
	username := "foo"
	password := "barbaz"

	err := s.a.UpsertPassword(username, []byte(password))
	c.Assert(err, IsNil)

	var elapsedExists time.Duration
	for i := 0; i < 10; i++ {
		start := time.Now()
		s.a.CheckPasswordWOToken(username, []byte(password))
		elapsed := time.Since(start)
		elapsedExists = elapsedExists + elapsed
	}

	var elapsedNotExists time.Duration
	for i := 0; i < 10; i++ {
		start := time.Now()
		s.a.CheckPasswordWOToken("blah", []byte(password))
		elapsed := time.Since(start)
		elapsedNotExists = elapsedNotExists + elapsed
	}

	// elapsedDifference must be less than 40 ms (with some leeway for delays in runtime)
	elapsedDifference := elapsedExists/10 - elapsedNotExists/10
	comment := Commentf("elapsed difference (%v) greater than 40 ms", elapsedDifference)
	c.Assert(elapsedDifference.Seconds() < 0.040, Equals, true, comment)
}

func (s *PasswordSuite) TestChangePassword(c *C) {
	req, err := s.prepareForPasswordChange("user1", []byte("abc123"), teleport.OFF)
	c.Assert(err, IsNil)

	fakeClock := clockwork.NewFakeClock()
	s.a.SetClock(fakeClock)
	req.NewPassword = []byte("abce456")

	// test change password event
	eventEmitted := false
	s.mockedAuditLog.MockEmitAuditEvent = func(event events.Event, fields events.EventFields) error {
		eventEmitted = true
		c.Assert(event, DeepEquals, events.UserPasswordChange)
		c.Assert(fields[events.EventUser], Equals, "user1")
		return nil
	}

	err = s.a.ChangePassword(req)
	c.Assert(err, IsNil)
	c.Assert(eventEmitted, Equals, true)

	s.shouldLockAfterFailedAttempts(c, req)

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)
	req.OldPassword = req.NewPassword
	req.NewPassword = []byte("abc5555")
	err = s.a.ChangePassword(req)
	c.Assert(err, IsNil)
}

func (s *PasswordSuite) TestChangePasswordWithOTP(c *C) {
	req, err := s.prepareForPasswordChange("user2", []byte("abc123"), teleport.OTP)
	c.Assert(err, IsNil)

	otpSecret := base32.StdEncoding.EncodeToString([]byte("def456"))
	err = s.a.UpsertTOTP(req.User, otpSecret)
	c.Assert(err, IsNil)

	fakeClock := clockwork.NewFakeClock()
	s.a.SetClock(fakeClock)

	validToken, err := totp.GenerateCode(otpSecret, s.a.GetClock().Now())
	c.Assert(err, IsNil)

	// change password
	req.NewPassword = []byte("abce456")
	req.SecondFactorToken = validToken
	err = s.a.ChangePassword(req)
	c.Assert(err, IsNil)

	s.shouldLockAfterFailedAttempts(c, req)

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)

	validToken, _ = totp.GenerateCode(otpSecret, s.a.GetClock().Now())
	req.OldPassword = req.NewPassword
	req.NewPassword = []byte("abc5555")
	req.SecondFactorToken = validToken
	err = s.a.ChangePassword(req)
	c.Assert(err, IsNil)
}

func (s *PasswordSuite) TestChangePasswordWithToken(c *C) {
	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OFF,
	})
	c.Assert(err, IsNil)

	err = s.a.SetAuthPreference(authPreference)
	c.Assert(err, IsNil)

	username := "joe@example.com"
	password := []byte("qweqweqwe")
	_, _, err = CreateUserAndRole(s.a, username, []string{username})
	c.Assert(err, IsNil)

	token, err := s.a.CreateResetPasswordToken(context.TODO(), CreateResetPasswordTokenRequest{
		Name: username,
	})
	c.Assert(err, IsNil)

	_, err = s.a.changePasswordWithToken(context.TODO(), ChangePasswordWithTokenRequest{
		TokenID:  token.GetName(),
		Password: password,
	})
	c.Assert(err, IsNil)

	// password should be updated
	err = s.a.CheckPasswordWOToken(username, password)
	c.Assert(err, IsNil)
}

func (s *PasswordSuite) TestChangePasswordWithTokenOTP(c *C) {
	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OTP,
	})
	c.Assert(err, IsNil)

	err = s.a.SetAuthPreference(authPreference)
	c.Assert(err, IsNil)

	username := "joe@example.com"
	password := []byte("qweqweqwe")
	_, _, err = CreateUserAndRole(s.a, username, []string{username})
	c.Assert(err, IsNil)

	token, err := s.a.CreateResetPasswordToken(context.TODO(), CreateResetPasswordTokenRequest{
		Name: username,
	})
	c.Assert(err, IsNil)

	secrets, err := s.a.RotateResetPasswordTokenSecrets(context.TODO(), token.GetName())
	c.Assert(err, IsNil)

	otpToken, err := totp.GenerateCode(secrets.GetOTPKey(), s.bk.Clock().Now())
	c.Assert(err, IsNil)

	_, err = s.a.changePasswordWithToken(context.TODO(), ChangePasswordWithTokenRequest{
		TokenID:           token.GetName(),
		Password:          password,
		SecondFactorToken: otpToken,
	})
	c.Assert(err, IsNil)

	err = s.a.CheckPasswordWOToken(username, password)
	c.Assert(err, IsNil)
}

func (s *PasswordSuite) TestChangePasswordWithTokenErrors(c *C) {
	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OTP,
	})
	c.Assert(err, IsNil)

	username := "joe@example.com"
	_, _, err = CreateUserAndRole(s.a, username, []string{username})
	c.Assert(err, IsNil)

	token, err := s.a.CreateResetPasswordToken(context.TODO(), CreateResetPasswordTokenRequest{
		Name: username,
	})
	c.Assert(err, IsNil)

	validPassword := []byte("qweQWE1")
	validTokenID := token.GetName()

	type testCase struct {
		desc         string
		secondFactor string
		req          ChangePasswordWithTokenRequest
	}

	testCases := []testCase{
		{
			secondFactor: teleport.OFF,
			desc:         "invalid tokenID value",
			req: ChangePasswordWithTokenRequest{
				TokenID:  "what_token",
				Password: validPassword,
			},
		},
		{
			secondFactor: teleport.OFF,
			desc:         "invalid password",
			req: ChangePasswordWithTokenRequest{
				TokenID:  validTokenID,
				Password: []byte("short"),
			},
		},
		{
			secondFactor: teleport.OTP,
			desc:         "missing second factor",
			req: ChangePasswordWithTokenRequest{
				TokenID:  validTokenID,
				Password: validPassword,
			},
		},
		{
			secondFactor: teleport.OTP,
			desc:         "invalid OTP value",
			req: ChangePasswordWithTokenRequest{
				TokenID:           validTokenID,
				Password:          validPassword,
				SecondFactorToken: "invalid",
			},
		},
	}

	for _, tc := range testCases {
		// set new auth preference settings
		authPreference.SetSecondFactor(tc.secondFactor)
		err = s.a.SetAuthPreference(authPreference)
		c.Assert(err, IsNil)

		_, err = s.a.changePasswordWithToken(context.TODO(), tc.req)
		c.Assert(err, NotNil, Commentf("test case %q", tc.desc))
	}

	authPreference.SetSecondFactor(teleport.OFF)
	err = s.a.SetAuthPreference(authPreference)
	c.Assert(err, IsNil)

	_, err = s.a.changePasswordWithToken(context.TODO(), ChangePasswordWithTokenRequest{
		TokenID:  validTokenID,
		Password: validPassword,
	})
	c.Assert(err, IsNil)

	// invite token cannot be reused
	_, err = s.a.changePasswordWithToken(context.TODO(), ChangePasswordWithTokenRequest{
		TokenID:  validTokenID,
		Password: validPassword,
	})
	c.Assert(err, NotNil)
}

func (s *PasswordSuite) shouldLockAfterFailedAttempts(c *C, req services.ChangePasswordReq) {
	loginAttempts, _ := s.a.GetUserLoginAttempts(req.User)
	c.Assert(len(loginAttempts), Equals, 0)
	for i := 0; i < defaults.MaxLoginAttempts; i++ {
		err := s.a.ChangePassword(req)
		c.Assert(err, NotNil)
		loginAttempts, _ = s.a.GetUserLoginAttempts(req.User)
		c.Assert(len(loginAttempts), Equals, i+1)
	}

	err := s.a.ChangePassword(req)
	c.Assert(trace.IsAccessDenied(err), Equals, true)
}

func (s *PasswordSuite) prepareForPasswordChange(user string, pass []byte, secondFactorType string) (services.ChangePasswordReq, error) {
	req := services.ChangePasswordReq{
		User:        user,
		OldPassword: pass,
	}

	err := s.a.UpsertCertAuthority(suite.NewTestCA(services.UserCA, "me.localhost"))
	if err != nil {
		return req, err
	}

	err = s.a.UpsertCertAuthority(suite.NewTestCA(services.HostCA, "me.localhost"))
	if err != nil {
		return req, err
	}

	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: secondFactorType,
	})
	if err != nil {
		return req, err
	}

	err = s.a.SetAuthPreference(ap)
	if err != nil {
		return req, err
	}

	_, _, err = CreateUserAndRole(s.a, user, []string{user})
	if err != nil {
		return req, err
	}
	err = s.a.UpsertPassword(user, pass)
	if err != nil {
		return req, err
	}

	return req, nil
}
