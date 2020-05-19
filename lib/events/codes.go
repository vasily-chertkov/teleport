/*
Copyright 2019 Gravitational, Inc.

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

package events

// Event describes an audit log event.
type Event struct {
	// Name is the event name.
	Name string
	// Code is the unique event code.
	Code string
}

var (
	// UserLocalLogin is emitted when a local user successfully logs in.
	UserLocalLogin = Event{
		Name: UserLoginEvent,
		Code: UserLocalLoginCode,
	}
	// UserLocalLoginFailure is emitted when a local user login attempt fails.
	UserLocalLoginFailure = Event{
		Name: UserLoginEvent,
		Code: UserLocalLoginFailureCode,
	}
	// UserSSOLogin is emitted when an SSO user successfully logs in.
	UserSSOLogin = Event{
		Name: UserLoginEvent,
		Code: UserSSOLoginCode,
	}
	// UserSSOLoginFailure is emitted when an SSO user login attempt fails.
	UserSSOLoginFailure = Event{
		Name: UserLoginEvent,
		Code: UserSSOLoginFailureCode,
	}
	// UserUpdate is emitted when a user is updated.
	UserUpdate = Event{
		Name: UserUpdatedEvent,
		Code: UserUpdateCode,
	}
	// UserDelete is emitted when a user is deleted.
	UserDelete = Event{
		Name: UserDeleteEvent,
		Code: UserDeleteCode,
	}
	// UserCreate is emitted when a user is created.
	UserCreate = Event{
		Name: UserCreateEvent,
		Code: UserCreateCode,
	}
	// UserPasswordChange is emitted when a user changes their own password.
	UserPasswordChange = Event{
		Name: UserPasswordChangeEvent,
		Code: UserPasswordChangeCode,
	}
	// SessionStart is emitted when a user starts a new session.
	SessionStart = Event{
		Name: SessionStartEvent,
		Code: SessionStartCode,
	}
	// SessionJoin is emitted when a user joins the session.
	SessionJoin = Event{
		Name: SessionJoinEvent,
		Code: SessionJoinCode,
	}
	// TerminalResize is emitted when a user resizes the terminal.
	TerminalResize = Event{
		Name: ResizeEvent,
		Code: TerminalResizeCode,
	}
	// SessionLeave is emitted when a user leaves the session.
	SessionLeave = Event{
		Name: SessionLeaveEvent,
		Code: SessionLeaveCode,
	}
	// SessionEnd is emitted when a user ends the session.
	SessionEnd = Event{
		Name: SessionEndEvent,
		Code: SessionEndCode,
	}
	// SessionUpload is emitted after a session recording has been uploaded.
	SessionUpload = Event{
		Name: SessionUploadEvent,
		Code: SessionUploadCode,
	}
	// SessionData is emitted to report session data usage.
	SessionData = Event{
		Name: SessionDataEvent,
		Code: SessionDataCode,
	}
	// Subsystem is emitted when a user requests a new subsystem.
	Subsystem = Event{
		Name: SubsystemEvent,
		Code: SubsystemCode,
	}
	// SubsystemFailure is emitted when a user subsystem request fails.
	SubsystemFailure = Event{
		Name: SubsystemEvent,
		Code: SubsystemFailureCode,
	}
	// Exec is emitted when a user executes a command on a node.
	Exec = Event{
		Name: ExecEvent,
		Code: ExecCode,
	}
	// ExecFailure is emitted when a user command execution fails.
	ExecFailure = Event{
		Name: ExecEvent,
		Code: ExecFailureCode,
	}
	// PortForward is emitted when a user requests port forwarding.
	PortForward = Event{
		Name: PortForwardEvent,
		Code: PortForwardCode,
	}
	// PortForwardFailure is emitted when a port forward request fails.
	PortForwardFailure = Event{
		Name: PortForwardEvent,
		Code: PortForwardFailureCode,
	}
	// SCPDownload is emitted when a user downloads a file.
	SCPDownload = Event{
		Name: SCPEvent,
		Code: SCPDownloadCode,
	}
	// SCPDownloadFailure is emitted when a file download fails.
	SCPDownloadFailure = Event{
		Name: SCPEvent,
		Code: SCPDownloadFailureCode,
	}
	// SCPUpload is emitted when a user uploads a file.
	SCPUpload = Event{
		Name: SCPEvent,
		Code: SCPUploadCode,
	}
	// SCPUploadFailure is emitted when a file upload fails.
	SCPUploadFailure = Event{
		Name: SCPEvent,
		Code: SCPUploadFailureCode,
	}
	// ClientDisconnect is emitted when a user session is disconnected.
	ClientDisconnect = Event{
		Name: ClientDisconnectEvent,
		Code: ClientDisconnectCode,
	}
	// AuthAttemptFailure is emitted upon a failed authentication attempt.
	AuthAttemptFailure = Event{
		Name: AuthAttemptEvent,
		Code: AuthAttemptFailureCode,
	}
	// AccessRequestCreated is emitted when an access request is created.
	AccessRequestCreated = Event{
		Name: AccessRequestCreateEvent,
		Code: AccessRequestCreateCode,
	}
	AccessRequestUpdated = Event{
		Name: AccessRequestUpdateEvent,
		Code: AccessRequestUpdateCode,
	}
	// SessionCommand is emitted upon execution of a command when using enhanced
	// session recording.
	SessionCommand = Event{
		Name: SessionCommandEvent,
		Code: SessionCommandCode,
	}
	// SessionDisk is emitted upon open of a file when using enhanced session recording.
	SessionDisk = Event{
		Name: SessionDiskEvent,
		Code: SessionDiskCode,
	}
	// SessionNetwork is emitted when a network requests is is issued when
	// using enhanced session recording.
	SessionNetwork = Event{
		Name: SessionNetworkEvent,
		Code: SessionNetworkCode,
	}
	// ResetPasswordTokenCreated is emitted when token is created.
	ResetPasswordTokenCreated = Event{
		Name: ResetPasswordTokenCreateEvent,
		Code: ResetPasswordTokenCreateCode,
	}
	// RoleCreated is emitted when a role is created/updated.
	RoleCreated = Event{
		Name: RoleCreatedEvent,
		Code: RoleCreatedCode,
	}
	// RoleDeleted is emitted when a role is deleted.
	RoleDeleted = Event{
		Name: RoleDeletedEvent,
		Code: RoleDeletedCode,
	}
	// GithubConnectorCreated is emitted when a Github connector is created/updated.
	GithubConnectorCreated = Event{
		Name: GithubConnectorCreatedEvent,
		Code: GithubConnectorCreatedCode,
	}
	// GithubConnectorDeleted is emitted when a Github connector is deleted.
	GithubConnectorDeleted = Event{
		Name: GithubConnectorDeletedEvent,
		Code: GithubConnectorDeletedCode,
	}
	// OIDCConnectorCreated is emitted when an OIDC connector is created/updated.
	OIDCConnectorCreated = Event{
		Name: OIDCConnectorCreatedEvent,
		Code: OIDCConnectorCreatedCode,
	}
	// OIDCConnectorDeleted is emitted when an OIDC connector is deleted.
	OIDCConnectorDeleted = Event{
		Name: OIDCConnectorDeletedEvent,
		Code: OIDCConnectorDeletedCode,
	}
	// SAMLConnectorCreated is emitted when a SAML connector is created/updated.
	SAMLConnectorCreated = Event{
		Name: SAMLConnectorCreatedEvent,
		Code: SAMLConnectorCreatedCode,
	}
	// SAMLConnectorDeleted is emitted when a SAML connector is deleted.
	SAMLConnectorDeleted = Event{
		Name: SAMLConnectorDeletedEvent,
		Code: SAMLConnectorDeletedCode,
	}
)

// OSS event codes start with "T".
const (
	// UserLocalLoginCode is the successful local user login event code.
	UserLocalLoginCode = "T1000I"
	// UserLocalLoginFailureCode is the unsuccessful local user login event code.
	UserLocalLoginFailureCode = "T1000W"
	// UserSSOLoginCode is the successful SSO user login event code.
	UserSSOLoginCode = "T1001I"
	// UserSSOLoginFailureCode is the unsuccessful SSO user login event code.
	UserSSOLoginFailureCode = "T1001W"
	// UserUpdateCode is the user update event code.
	UserUpdateCode = "T1002I"
	// UserDeleteCode is the user delete event code.
	UserDeleteCode = "T1003I"
	// UserCreateCode is the user create event code.
	UserCreateCode = "T1004I"
	// UserPasswordChangeCode is an event code for when user changes their own password.
	UserPasswordChangeCode = "T1005I"
	// SessionStartCode is the session start event code.
	SessionStartCode = "T2000I"
	// SessionJoinCode is the session join event code.
	SessionJoinCode = "T2001I"
	// TerminalResizeCode is the terminal resize event code.
	TerminalResizeCode = "T2002I"
	// SessionLeaveCode is the session leave event code.
	SessionLeaveCode = "T2003I"
	// SessionEndCode is the session end event code.
	SessionEndCode = "T2004I"
	// SessionUploadCode is the session upload event code.
	SessionUploadCode = "T2005I"
	// SessionDataCode is the session data event code.
	SessionDataCode = "T2006I"
	// SubsystemCode is the subsystem event code.
	SubsystemCode = "T3001I"
	// SubsystemFailureCode is the subsystem failure event code.
	SubsystemFailureCode = "T3001E"
	// ExecCode is the exec event code.
	ExecCode = "T3002I"
	// ExecFailureCode is the exec failure event code.
	ExecFailureCode = "T3002E"
	// PortForwardCode is the port forward event code.
	PortForwardCode = "T3003I"
	// PortForwardFailureCode is the port forward failure event code.
	PortForwardFailureCode = "T3003E"
	// SCPDownloadCode is the file download event code.
	SCPDownloadCode = "T3004I"
	// SCPDownloadFailureCode is the file download event failure code.
	SCPDownloadFailureCode = "T3004E"
	// SCPUploadCode is the file upload event code.
	SCPUploadCode = "T3005I"
	// SCPUploadFailureCode is the file upload failure event code.
	SCPUploadFailureCode = "T3005E"
	// ClientDisconnectCode is the client disconnect event code.
	ClientDisconnectCode = "T3006I"
	// AuthAttemptFailureCode is the auth attempt failure event code.
	AuthAttemptFailureCode = "T3007W"
	// SessionCommandCode is a session command code.
	SessionCommandCode = "T4000I"
	// SessionDiskCode is a session disk code.
	SessionDiskCode = "T4001I"
	// SessionNetworkCode is a session network code.
	SessionNetworkCode = "T4002I"
	// AccessRequestCreateCode is the the access request creation code.
	AccessRequestCreateCode = "T5000I"
	// AccessRequestUpdateCode is the access request state update code.
	AccessRequestUpdateCode = "T5001I"
	// ResetPasswordTokenCreateCode is the token create event code.
	ResetPasswordTokenCreateCode = "T6000I"
	// GithubConnectorCreatedCode is the Github connector created event code.
	GithubConnectorCreatedCode = "T8001I"
	// GithubConnectorDeletedCode is the Github connector deleted event code.
	GithubConnectorDeletedCode = "T8002I"
)

// Enterprise event codes starts with "TE".
const (
	// RoleCreatedCode is the role created event code.
	RoleCreatedCode = "TE1000I"
	// RoleDeletedCode is the role deleted event code.
	RoleDeletedCode = "TE2000I"
	// OIDCConnectorCreatedCode is the OIDC connector created event code.
	OIDCConnectorCreatedCode = "TE1001I"
	// OIDCConnectorDeletedCode is the OIDC connector deleted event code.
	OIDCConnectorDeletedCode = "TE2001I"
	// SAMLConnectorCreatedCode is the SAML connector created event code.
	SAMLConnectorCreatedCode = "TE1002I"
	// SAMLConnectorDeletedCode is the SAML connector deleted event code.
	SAMLConnectorDeletedCode = "TE2002I"
)
