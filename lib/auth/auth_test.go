/*
Copyright 2015-2019 Gravitational, Inc.

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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-oidc/jose"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	. "gopkg.in/check.v1"
)

func TestAPI(t *testing.T) { TestingT(t) }

type AuthSuite struct {
	bk             backend.Backend
	a              *AuthServer
	dataDir        string
	mockedAuditLog *events.MockAuditLog
}

var _ = Suite(&AuthSuite{})
var _ = fmt.Printf

func (s *AuthSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests(testing.Verbose())
}

func (s *AuthSuite) SetUpTest(c *C) {
	var err error
	s.dataDir = c.MkDir()
	s.bk, err = lite.NewWithConfig(context.TODO(), lite.Config{Path: s.dataDir})
	c.Assert(err, IsNil)

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

	// set cluster name
	err = s.a.SetClusterName(clusterName)
	c.Assert(err, IsNil)

	// set static tokens
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionTokenV1{},
	})
	c.Assert(err, IsNil)
	err = s.a.SetStaticTokens(staticTokens)
	c.Assert(err, IsNil)

	authPreference, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OFF,
	})
	c.Assert(err, IsNil)

	err = s.a.SetAuthPreference(authPreference)
	c.Assert(err, IsNil)

	err = s.a.SetClusterConfig(services.DefaultClusterConfig())
	c.Assert(err, IsNil)

	s.mockedAuditLog = events.NewMockAuditLog(0)
	s.a.IAuditLog = s.mockedAuditLog
}

func (s *AuthSuite) TearDownTest(c *C) {
	if s.bk != nil {
		s.bk.Close()
	}
}

func (s *AuthSuite) TestSessions(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "me.localhost")), IsNil)

	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.HostCA, "me.localhost")), IsNil)

	user := "user1"
	pass := []byte("abc123")

	_, err := s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass:     &PassCreds{Password: pass},
	})
	c.Assert(err, NotNil)

	_, _, err = CreateUserAndRole(s.a, user, []string{user})
	c.Assert(err, IsNil)

	err = s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	ws, err := s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass:     &PassCreds{Password: pass},
	})
	c.Assert(err, IsNil)
	c.Assert(ws, NotNil)

	out, err := s.a.GetWebSessionInfo(user, ws.GetName())
	c.Assert(err, IsNil)
	ws.SetPriv(nil)
	fixtures.DeepCompare(c, ws, out)

	err = s.a.DeleteWebSession(user, ws.GetName())
	c.Assert(err, IsNil)

	_, err = s.a.GetWebSession(user, ws.GetName())
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
}

func (s *AuthSuite) TestUserLock(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.UserCA, "me.localhost")), IsNil)

	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.HostCA, "me.localhost")), IsNil)

	user := "user1"
	pass := []byte("abc123")

	_, err := s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass:     &PassCreds{Password: pass},
	})
	c.Assert(err, NotNil)

	_, _, err = CreateUserAndRole(s.a, user, []string{user})
	c.Assert(err, IsNil)

	err = s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	// successful log in
	ws, err := s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass:     &PassCreds{Password: pass},
	})
	c.Assert(err, IsNil)
	c.Assert(ws, NotNil)

	fakeClock := clockwork.NewFakeClock()
	s.a.SetClock(fakeClock)

	for i := 0; i <= defaults.MaxLoginAttempts; i++ {
		_, err = s.a.AuthenticateWebUser(AuthenticateUserRequest{
			Username: user,
			Pass:     &PassCreds{Password: []byte("wrong pass")},
		})
		c.Assert(err, NotNil)
	}

	// make sure user is locked
	_, err = s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass:     &PassCreds{Password: pass},
	})
	c.Assert(err, ErrorMatches, ".*locked.*")

	// advance time and make sure we can login again
	fakeClock.Advance(defaults.AccountLockInterval + time.Second)

	_, err = s.a.AuthenticateWebUser(AuthenticateUserRequest{
		Username: user,
		Pass:     &PassCreds{Password: pass},
	})
	c.Assert(err, IsNil)
}

func (s *AuthSuite) TestTokensCRUD(c *C) {
	c.Assert(s.a.UpsertCertAuthority(
		suite.NewTestCA(services.HostCA, "me.localhost")), IsNil)

	// before we do anything, we should have 0 tokens
	btokens, err := s.a.GetTokens()
	c.Assert(err, IsNil)
	c.Assert(len(btokens), Equals, 0)

	// generate persistent token
	tok, err := s.a.GenerateToken(GenerateTokenRequest{Roles: teleport.Roles{teleport.RoleNode}})
	c.Assert(err, IsNil)
	c.Assert(len(tok), Equals, 2*TokenLenBytes)

	tokens, err := s.a.GetTokens()
	c.Assert(err, IsNil)
	c.Assert(len(tokens), Equals, 1)
	c.Assert(tokens[0].GetName(), Equals, tok)

	roles, err := s.a.ValidateToken(tok)
	c.Assert(err, IsNil)
	c.Assert(roles.Include(teleport.RoleNode), Equals, true)
	c.Assert(roles.Include(teleport.RoleProxy), Equals, false)

	// unsuccessful registration (wrong role)
	keys, err := s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    tok,
		HostID:   "bad-host-id",
		NodeName: "bad-node-name",
		Role:     teleport.RoleProxy,
	})
	c.Assert(keys, IsNil)
	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, `node "bad-node-name" \[bad-host-id\] can not join the cluster, the token does not allow "Proxy" role`)

	// generate predefined token
	customToken := "custom-token"
	tok, err = s.a.GenerateToken(GenerateTokenRequest{Roles: teleport.Roles{teleport.RoleNode}, Token: customToken})
	c.Assert(err, IsNil)
	c.Assert(tok, Equals, customToken)

	roles, err = s.a.ValidateToken(tok)
	c.Assert(err, IsNil)
	c.Assert(roles.Include(teleport.RoleNode), Equals, true)
	c.Assert(roles.Include(teleport.RoleProxy), Equals, false)

	err = s.a.DeleteToken(customToken)
	c.Assert(err, IsNil)

	// generate multi-use token with long TTL:
	multiUseToken, err := s.a.GenerateToken(GenerateTokenRequest{Roles: teleport.Roles{teleport.RoleProxy}, TTL: time.Hour})
	c.Assert(err, IsNil)
	_, err = s.a.ValidateToken(multiUseToken)
	c.Assert(err, IsNil)

	// use it twice:
	keys, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:                multiUseToken,
		HostID:               "once",
		NodeName:             "node-name",
		Role:                 teleport.RoleProxy,
		AdditionalPrincipals: []string{"example.com"},
	})
	c.Assert(err, IsNil)

	// along the way, make sure that additional principals work
	key, _, _, _, err := ssh.ParseAuthorizedKey(keys.Cert)
	c.Assert(err, IsNil)
	hostCert := key.(*ssh.Certificate)
	comment := Commentf("can't find example.com in %v", hostCert.ValidPrincipals)
	c.Assert(utils.SliceContainsStr(hostCert.ValidPrincipals, "example.com"), Equals, true, comment)

	_, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    multiUseToken,
		HostID:   "twice",
		NodeName: "node-name",
		Role:     teleport.RoleProxy,
	})
	c.Assert(err, IsNil)

	// try to use after TTL:
	s.a.SetClock(clockwork.NewFakeClockAt(time.Now().UTC().Add(time.Hour + 1)))
	_, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    multiUseToken,
		HostID:   "late.bird",
		NodeName: "node-name",
		Role:     teleport.RoleProxy,
	})
	c.Assert(err, ErrorMatches, `"node-name" \[late.bird\] can not join the cluster with role Proxy, the token is not valid`)

	// expired token should be gone now
	err = s.a.DeleteToken(multiUseToken)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	// lets use static tokens now
	roles = teleport.Roles{teleport.RoleProxy}
	st, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionTokenV1{services.ProvisionTokenV1{
			Token:   "static-token-value",
			Roles:   roles,
			Expires: time.Unix(0, 0).UTC(),
		}},
	})
	c.Assert(err, IsNil)
	err = s.a.SetStaticTokens(st)
	c.Assert(err, IsNil)
	_, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    "static-token-value",
		HostID:   "static.host",
		NodeName: "node-name",
		Role:     teleport.RoleProxy,
	})
	c.Assert(err, IsNil)
	_, err = s.a.RegisterUsingToken(RegisterUsingTokenRequest{
		Token:    "static-token-value",
		HostID:   "wrong.role",
		NodeName: "node-name",
		Role:     teleport.RoleAuth,
	})
	c.Assert(err, NotNil)
	r, err := s.a.ValidateToken("static-token-value")
	c.Assert(err, IsNil)
	c.Assert(r, DeepEquals, roles)

	// List tokens (should see 2: one static, one regular)
	tokens, err = s.a.GetTokens()
	c.Assert(err, IsNil)
	c.Assert(len(tokens), Equals, 2)
}

func (s *AuthSuite) TestBadTokens(c *C) {
	// empty
	_, err := s.a.ValidateToken("")
	c.Assert(err, NotNil)

	// garbage
	_, err = s.a.ValidateToken("bla bla")
	c.Assert(err, NotNil)

	// tampered
	tok, err := s.a.GenerateToken(GenerateTokenRequest{Roles: teleport.Roles{teleport.RoleAuth}})
	c.Assert(err, IsNil)

	tampered := string(tok[0]+1) + tok[1:]
	_, err = s.a.ValidateToken(tampered)
	c.Assert(err, NotNil)
}

func (s *AuthSuite) TestBuildRolesInvalid(c *C) {
	// create a connector
	oidcConnector := services.NewOIDCConnector("example", services.OIDCConnectorSpecV2{
		IssuerURL:    "https://www.exmaple.com",
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURL:  "https://localhost:3080/v1/webapi/oidc/callback",
		Display:      "sign in with example.com",
		Scope:        []string{"foo", "bar"},
	})

	// create some claims
	var claims = make(jose.Claims)
	claims.Add("roles", "teleport-user")
	claims.Add("email", "foo@example.com")
	claims.Add("nickname", "foo")
	claims.Add("full_name", "foo bar")

	// try and build roles should be invalid since we have no mappings
	_, err := s.a.buildOIDCRoles(oidcConnector, claims)
	c.Assert(err, NotNil)
}

func (s *AuthSuite) TestBuildRolesStatic(c *C) {
	// create a connector
	oidcConnector := services.NewOIDCConnector("example", services.OIDCConnectorSpecV2{
		IssuerURL:    "https://www.exmaple.com",
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURL:  "https://localhost:3080/v1/webapi/oidc/callback",
		Display:      "sign in with example.com",
		Scope:        []string{"foo", "bar"},
		ClaimsToRoles: []services.ClaimMapping{
			services.ClaimMapping{
				Claim: "roles",
				Value: "teleport-user",
				Roles: []string{"user"},
			},
		},
	})

	// create some claims
	var claims = make(jose.Claims)
	claims.Add("roles", "teleport-user")
	claims.Add("email", "foo@example.com")
	claims.Add("nickname", "foo")
	claims.Add("full_name", "foo bar")

	// build roles and check that we mapped to "user" role
	roles, err := s.a.buildOIDCRoles(oidcConnector, claims)
	c.Assert(err, IsNil)
	c.Assert(roles, HasLen, 1)
	c.Assert(roles[0], Equals, "user")
}

func (s *AuthSuite) TestValidateACRValues(c *C) {

	var tests = []struct {
		inIDToken     string
		inACRValue    string
		inACRProvider string
		outIsValid    bool
	}{
		// 0 - default, acr values match
		{
			`
{
	"acr": "foo",
	"aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo",
			"",
			true,
		},
		// 1 - default, acr values do not match
		{
			`
{
	"acr": "foo",
	"aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"bar",
			"",
			false,
		},
		// 2 - netiq, acr values match
		{
			`
{
    "acr": {
        "values": [
            "foo/bar/baz"
        ]
    },
    "aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo/bar/baz",
			"netiq",
			true,
		},
		// 3 - netiq, invalid format
		{
			`
{
    "acr": {
        "values": "foo/bar/baz"
    },
    "aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo/bar/baz",
			"netiq",
			false,
		},
		// 4 - netiq, invalid value
		{
			`
{
    "acr": {
        "values": [
            "foo/bar/baz/qux"
        ]
    },
    "aud": "00000000-0000-0000-0000-000000000000",
    "exp": 1111111111
}
			`,
			"foo/bar/baz",
			"netiq",
			false,
		},
	}

	for i, tt := range tests {
		comment := Commentf("Test %v", i)

		var claims jose.Claims
		err := json.Unmarshal([]byte(tt.inIDToken), &claims)
		c.Assert(err, IsNil, comment)

		err = s.a.validateACRValues(tt.inACRValue, tt.inACRProvider, claims)
		if tt.outIsValid {
			c.Assert(err, IsNil, comment)
		} else {
			c.Assert(err, NotNil, comment)
		}
	}
}

func (s *AuthSuite) TestUpdateConfig(c *C) {
	cn, err := s.a.GetClusterName()
	c.Assert(err, IsNil)
	c.Assert(cn.GetClusterName(), Equals, "me.localhost")
	st, err := s.a.GetStaticTokens()
	c.Assert(err, IsNil)
	c.Assert(st.GetStaticTokens(), DeepEquals, []services.ProvisionToken{})

	// try and set cluster name, this should fail because you can only set the
	// cluster name once
	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "foo.localhost",
	})
	c.Assert(err, IsNil)
	// use same backend but start a new auth server with different config.
	authConfig := &InitConfig{
		ClusterName:            clusterName,
		Backend:                s.bk,
		Authority:              authority.New(),
		SkipPeriodicOperations: true,
	}
	authServer, err := NewAuthServer(authConfig)
	c.Assert(err, IsNil)

	err = authServer.SetClusterName(clusterName)
	c.Assert(err, NotNil)
	// try and set static tokens, this should be successful because the last
	// one to upsert tokens wins
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionTokenV1{services.ProvisionTokenV1{
			Token: "bar",
			Roles: teleport.Roles{teleport.Role("baz")},
		}},
	})
	c.Assert(err, IsNil)
	err = authServer.SetStaticTokens(staticTokens)
	c.Assert(err, IsNil)

	// check first auth server and make sure it returns the correct values
	// (original cluster name, new static tokens)
	cn, err = s.a.GetClusterName()
	c.Assert(err, IsNil)
	c.Assert(cn.GetClusterName(), Equals, "me.localhost")
	st, err = s.a.GetStaticTokens()
	c.Assert(err, IsNil)
	c.Assert(st.GetStaticTokens(), DeepEquals, services.ProvisionTokensFromV1([]services.ProvisionTokenV1{services.ProvisionTokenV1{
		Token: "bar",
		Roles: teleport.Roles{teleport.Role("baz")},
	}}))

	// check second auth server and make sure it also has the correct values
	// new static tokens
	st, err = authServer.GetStaticTokens()
	c.Assert(err, IsNil)
	c.Assert(st.GetStaticTokens(), DeepEquals, services.ProvisionTokensFromV1([]services.ProvisionTokenV1{services.ProvisionTokenV1{
		Token: "bar",
		Roles: teleport.Roles{teleport.Role("baz")},
	}}))
}

func (s *AuthSuite) TestUpdateByWithContext(c *C) {
	ctx := withUpdateBy(context.TODO(), "some-user")
	updatedBy, err := getUpdateBy(ctx)
	c.Assert(err, IsNil)
	c.Assert(updatedBy, Equals, "some-user")

	// trigger error
	ctx = withUpdateBy(context.TODO(), "")
	updatedBy, err = getUpdateBy(ctx)
	c.Assert(trace.IsBadParameter(err), Equals, true)
	c.Assert(err, ErrorMatches, `missing value "updated_by"`)
	c.Assert(updatedBy, Equals, "")
}

func (s *AuthSuite) TestCreateAndUpdateUser(c *C) {
	user, err := services.NewUser("some-user")
	c.Assert(err, IsNil)

	// test create user
	createEventEmitted := false
	s.mockedAuditLog.MockEmitAuditEvent = func(event events.Event, fields events.EventFields) error {
		createEventEmitted = true
		c.Assert(event, DeepEquals, events.UserCreate)
		c.Assert(fields[events.EventUser], Equals, "some-auth-user")
		return nil
	}

	// trigger error
	err = s.a.CreateUser(context.TODO(), user)
	c.Assert(err, ErrorMatches, `created by is not set for new user "some-user"`)
	c.Assert(createEventEmitted, Equals, false)

	// happy path
	user.SetCreatedBy(services.CreatedBy{
		User: services.UserRef{Name: "some-auth-user"},
	})
	err = s.a.CreateUser(context.TODO(), user)
	c.Assert(err, IsNil)
	c.Assert(createEventEmitted, Equals, true)

	// test update user
	updateEventEmitted := false
	s.mockedAuditLog.MockEmitAuditEvent = func(event events.Event, fields events.EventFields) error {
		updateEventEmitted = true
		c.Assert(event, DeepEquals, events.UserUpdate)
		c.Assert(fields[events.EventUser], Equals, "some-context-user")
		return nil
	}
	ctx := withUpdateBy(context.TODO(), "some-context-user")
	c.Assert(ctx, NotNil)
	err = s.a.UpdateUser(ctx, user)
	c.Assert(err, IsNil)
	c.Assert(updateEventEmitted, Equals, true)
}

func (s *AuthSuite) TestUpsertDeleteRole(c *C) {
	// test create new role
	roleTest, err := services.NewRole("test", services.RoleSpecV3{
		Options: services.RoleOptions{},
		Allow:   services.RoleConditions{},
	})
	c.Assert(err, IsNil)

	createEventEmitted := false
	s.mockedAuditLog.MockEmitAuditEvent = func(event events.Event, fields events.EventFields) error {
		createEventEmitted = true
		c.Assert(event, DeepEquals, events.RoleCreated)
		c.Assert(fields[events.FieldName], Equals, "test")
		return nil
	}

	err = s.a.upsertRole(roleTest)
	c.Assert(err, IsNil)
	c.Assert(createEventEmitted, Equals, true)

	roleRetrieved, err := s.a.GetRole("test")
	c.Assert(err, IsNil)
	c.Assert(roleRetrieved.Equals(roleTest), Equals, true)

	// test update role
	createEventEmitted = false
	err = s.a.upsertRole(roleTest)
	c.Assert(err, IsNil)
	c.Assert(roleRetrieved.Equals(roleTest), Equals, true)
	c.Assert(createEventEmitted, Equals, true)

	// test delete role
	deleteEventEmitted := false
	s.mockedAuditLog.MockEmitAuditEvent = func(event events.Event, fields events.EventFields) error {
		deleteEventEmitted = true
		c.Assert(event, DeepEquals, events.RoleDeleted)
		c.Assert(fields[events.FieldName], Equals, "test")
		return nil
	}

	err = s.a.DeleteRole("test")
	c.Assert(err, IsNil)
	c.Assert(deleteEventEmitted, Equals, true)

	// test role has been deleted
	roleRetrieved, err = s.a.GetRole("test")
	c.Assert(trace.IsNotFound(err), Equals, true)
	c.Assert(roleRetrieved, IsNil)

	// test role that doesn't exist
	deleteEventEmitted = false
	err = s.a.DeleteRole("test")
	c.Assert(trace.IsNotFound(err), Equals, true)
	c.Assert(deleteEventEmitted, Equals, false)

}

func (s *AuthSuite) TestGithubConnectorCRUDEventsEmitted(c *C) {
	eventEmitted := false
	s.mockedAuditLog.MockEmitAuditEvent = func(event events.Event, fields events.EventFields) error {
		eventEmitted = true
		c.Assert(event, DeepEquals, events.GithubConnectorCreated)
		return nil
	}

	// test github create event
	github := services.NewGithubConnector("test", services.GithubConnectorSpecV3{})
	err := s.a.upsertGithubConnector(github)
	c.Assert(err, IsNil)
	c.Assert(eventEmitted, Equals, true)

	// test github update event
	eventEmitted = false
	err = s.a.upsertGithubConnector(github)
	c.Assert(err, IsNil)
	c.Assert(eventEmitted, Equals, true)

	// test github delete event
	eventEmitted = false
	s.mockedAuditLog.MockEmitAuditEvent = func(event events.Event, fields events.EventFields) error {
		eventEmitted = true
		c.Assert(event, DeepEquals, events.GithubConnectorDeleted)
		return nil
	}

	err = s.a.deleteGithubConnector("test")
	c.Assert(err, IsNil)
	c.Assert(eventEmitted, Equals, true)
}

func (s *AuthSuite) TestOIDCConnectorCRUDEventsEmitted(c *C) {
	eventEmitted := false
	s.mockedAuditLog.MockEmitAuditEvent = func(event events.Event, fields events.EventFields) error {
		eventEmitted = true
		c.Assert(event, DeepEquals, events.OIDCConnectorCreated)
		return nil
	}

	// test oidc create event
	oidc := services.NewOIDCConnector("test", services.OIDCConnectorSpecV2{ClientID: "a"})
	err := s.a.UpsertOIDCConnector(oidc)
	c.Assert(err, IsNil)
	c.Assert(eventEmitted, Equals, true)

	// test oidc update event
	eventEmitted = false
	err = s.a.UpsertOIDCConnector(oidc)
	c.Assert(err, IsNil)
	c.Assert(eventEmitted, Equals, true)

	// test oidc delete event
	eventEmitted = false
	s.mockedAuditLog.MockEmitAuditEvent = func(event events.Event, fields events.EventFields) error {
		eventEmitted = true
		c.Assert(event, DeepEquals, events.OIDCConnectorDeleted)
		return nil
	}

	err = s.a.DeleteOIDCConnector("test")
	c.Assert(err, IsNil)
	c.Assert(eventEmitted, Equals, true)
}

func (s *AuthSuite) TestSAMLConnectorCRUDEventsEmitted(c *C) {
	eventEmitted := false
	s.mockedAuditLog.MockEmitAuditEvent = func(event events.Event, fields events.EventFields) error {
		eventEmitted = true
		c.Assert(event, DeepEquals, events.SAMLConnectorCreated)
		return nil
	}

	// generate a certificate that makes ParseCertificatePEM happy, copied from ca_test.go
	ca, err := tlsca.New([]byte(fixtures.SigningCertPEM), []byte(fixtures.SigningKeyPEM))
	c.Assert(err, IsNil)

	privateKey, err := rsa.GenerateKey(rand.Reader, teleport.RSAKeySize)
	c.Assert(err, IsNil)

	testClock := clockwork.NewFakeClock()
	certBytes, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     testClock,
		PublicKey: privateKey.Public(),
		Subject:   pkix.Name{CommonName: "test"},
		NotAfter:  testClock.Now().Add(time.Hour),
	})

	// test saml create
	saml := services.NewSAMLConnector("test", services.SAMLConnectorSpecV2{
		AssertionConsumerService: "a",
		Issuer:                   "b",
		SSO:                      "c",
		Cert:                     string(certBytes),
	})

	err = s.a.UpsertSAMLConnector(saml)
	c.Assert(err, IsNil)
	c.Assert(eventEmitted, Equals, true)

	// test saml update event
	eventEmitted = false
	err = s.a.UpsertSAMLConnector(saml)
	c.Assert(err, IsNil)
	c.Assert(eventEmitted, Equals, true)

	// test saml delete event
	eventEmitted = false
	s.mockedAuditLog.MockEmitAuditEvent = func(event events.Event, fields events.EventFields) error {
		eventEmitted = true
		c.Assert(event, DeepEquals, events.SAMLConnectorDeleted)
		return nil
	}

	err = s.a.DeleteSAMLConnector("test")
	c.Assert(err, IsNil)
	c.Assert(eventEmitted, Equals, true)
}
