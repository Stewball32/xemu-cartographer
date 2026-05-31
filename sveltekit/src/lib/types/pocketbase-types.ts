/**
* This file was @generated using pocketbase-typegen
*/

import type PocketBase from 'pocketbase'
import type { RecordService } from 'pocketbase'

export enum Collections {
	Authorigins = "_authOrigins",
	Externalauths = "_externalAuths",
	Mfas = "_mfas",
	Otps = "_otps",
	Superusers = "_superusers",
	AuditLog = "audit_log",
	CapturePolicies = "capture_policies",
	Containers = "containers",
	GameEvents = "game_events",
	Gamertags = "gamertags",
	Rosters = "rosters",
	Teams = "teams",
	Users = "users",
}

// Alias types for improved usability
export type IsoDateString = string
export type IsoAutoDateString = string & { readonly autodate: unique symbol }
export type RecordIdString = string
export type FileNameString = string & { readonly filename: unique symbol }
export type HTMLString = string

type ExpandType<T> = unknown extends T
	? T extends unknown
		? { expand?: unknown }
		: { expand: T }
	: { expand: T }

// System fields
export type BaseSystemFields<T = unknown> = {
	id: RecordIdString
	collectionId: string
	collectionName: Collections
} & ExpandType<T>

export type AuthSystemFields<T = unknown> = {
	email: string
	emailVisibility: boolean
	username: string
	verified: boolean
} & BaseSystemFields<T>

// Record types for each collection

export type AuthoriginsRecord = {
	collectionRef: string
	created: IsoAutoDateString
	fingerprint: string
	id: string
	recordRef: string
	updated: IsoAutoDateString
}

export type ExternalauthsRecord = {
	collectionRef: string
	created: IsoAutoDateString
	id: string
	provider: string
	providerId: string
	recordRef: string
	updated: IsoAutoDateString
}

export type MfasRecord = {
	collectionRef: string
	created: IsoAutoDateString
	id: string
	method: string
	recordRef: string
	updated: IsoAutoDateString
}

export type OtpsRecord = {
	collectionRef: string
	created: IsoAutoDateString
	id: string
	password: string
	recordRef: string
	sentTo?: string
	updated: IsoAutoDateString
}

export type SuperusersRecord = {
	created: IsoAutoDateString
	email: string
	emailVisibility?: boolean
	id: string
	password: string
	tokenKey: string
	updated: IsoAutoDateString
	verified?: boolean
}

export type AuditLogRecord<Tpayload_json = unknown> = {
	action: string
	actor?: RecordIdString
	created: IsoAutoDateString
	id: string
	payload_json?: null | Tpayload_json
	target_collection: string
	target_id: string
}

export enum CapturePoliciesModeOptions {
	"auto" = "auto",
	"always" = "always",
	"never" = "never",
}

export enum CapturePoliciesCadenceOptions {
	"default" = "default",
	"engine" = "engine",
	"30hz" = "30hz",
	"10hz" = "10hz",
	"5hz" = "5hz",
	"250ms" = "250ms",
	"500ms" = "500ms",
	"1s" = "1s",
}
export type CapturePoliciesRecord = {
	cadence: CapturePoliciesCadenceOptions
	class: string
	created: IsoAutoDateString
	description?: string
	id: string
	instance: string
	mode: CapturePoliciesModeOptions
	sink?: string
	updated: IsoAutoDateString
}

export type ContainersRecord = {
	browser_vnc: number
	browser_web: number
	created?: IsoDateString
	id: string
	index?: number
	name: string
	vnc_password?: string
	xemu_http: number
	xemu_https: number
	xemu_ws: number
}

export type GameEventsRecord<Tdata = unknown> = {
	created: IsoAutoDateString
	data?: null | Tdata
	id: string
	instance: string
	seq?: number
	tick?: number
	ts?: IsoDateString
	type: string
}

export enum GamertagsStatusOptions {
	"approved" = "approved",
	"allowed" = "allowed",
	"pending" = "pending",
	"blocked" = "blocked",
}
export type GamertagsRecord = {
	created: IsoAutoDateString
	id: string
	sanitized?: string
	status?: GamertagsStatusOptions
	tag: string
	updated: IsoAutoDateString
	user: RecordIdString
}

export type RostersRecord = {
	created: IsoAutoDateString
	gamertag: RecordIdString
	id: string
	is_manager?: boolean
	is_owner?: boolean
	joined_at: IsoDateString
	left_at?: IsoDateString
	team: RecordIdString
	updated: IsoAutoDateString
}

export enum TeamsStatusOptions {
	"approved" = "approved",
	"allowed" = "allowed",
	"pending" = "pending",
	"blocked" = "blocked",
}
export type TeamsRecord = {
	created: IsoAutoDateString
	created_by: RecordIdString
	id: string
	name: string
	slug: string
	status?: TeamsStatusOptions
	updated: IsoAutoDateString
}

export type UsersRecord = {
	avatar?: FileNameString
	bio?: string
	created: IsoAutoDateString
	default_gamertag?: RecordIdString
	deleted_at?: IsoDateString
	email: string
	emailVisibility?: boolean
	id: string
	isAdmin?: boolean
	is_deleted?: boolean
	location?: string
	name?: string
	password: string
	tokenKey: string
	updated: IsoAutoDateString
	username: string
	verified?: boolean
}

// Response types include system fields and match responses from the PocketBase API
export type AuthoriginsResponse<Texpand = unknown> = Required<AuthoriginsRecord> & BaseSystemFields<Texpand>
export type ExternalauthsResponse<Texpand = unknown> = Required<ExternalauthsRecord> & BaseSystemFields<Texpand>
export type MfasResponse<Texpand = unknown> = Required<MfasRecord> & BaseSystemFields<Texpand>
export type OtpsResponse<Texpand = unknown> = Required<OtpsRecord> & BaseSystemFields<Texpand>
export type SuperusersResponse<Texpand = unknown> = Required<SuperusersRecord> & AuthSystemFields<Texpand>
export type AuditLogResponse<Tpayload_json = unknown, Texpand = unknown> = Required<AuditLogRecord<Tpayload_json>> & BaseSystemFields<Texpand>
export type CapturePoliciesResponse<Texpand = unknown> = Required<CapturePoliciesRecord> & BaseSystemFields<Texpand>
export type ContainersResponse<Texpand = unknown> = Required<ContainersRecord> & BaseSystemFields<Texpand>
export type GameEventsResponse<Tdata = unknown, Texpand = unknown> = Required<GameEventsRecord<Tdata>> & BaseSystemFields<Texpand>
export type GamertagsResponse<Texpand = unknown> = Required<GamertagsRecord> & BaseSystemFields<Texpand>
export type RostersResponse<Texpand = unknown> = Required<RostersRecord> & BaseSystemFields<Texpand>
export type TeamsResponse<Texpand = unknown> = Required<TeamsRecord> & BaseSystemFields<Texpand>
export type UsersResponse<Texpand = unknown> = Required<UsersRecord> & AuthSystemFields<Texpand>

// Types containing all Records and Responses, useful for creating typing helper functions

export type CollectionRecords = {
	_authOrigins: AuthoriginsRecord
	_externalAuths: ExternalauthsRecord
	_mfas: MfasRecord
	_otps: OtpsRecord
	_superusers: SuperusersRecord
	audit_log: AuditLogRecord
	capture_policies: CapturePoliciesRecord
	containers: ContainersRecord
	game_events: GameEventsRecord
	gamertags: GamertagsRecord
	rosters: RostersRecord
	teams: TeamsRecord
	users: UsersRecord
}

export type CollectionResponses = {
	_authOrigins: AuthoriginsResponse
	_externalAuths: ExternalauthsResponse
	_mfas: MfasResponse
	_otps: OtpsResponse
	_superusers: SuperusersResponse
	audit_log: AuditLogResponse
	capture_policies: CapturePoliciesResponse
	containers: ContainersResponse
	game_events: GameEventsResponse
	gamertags: GamertagsResponse
	rosters: RostersResponse
	teams: TeamsResponse
	users: UsersResponse
}

// Utility types for create/update operations

type ProcessCreateAndUpdateFields<T> = Omit<{
	// Omit AutoDate fields
	[K in keyof T as Extract<T[K], IsoAutoDateString> extends never ? K : never]: 
		// Convert FileNameString to File
		T[K] extends infer U ? 
			U extends (FileNameString | FileNameString[]) ? 
				U extends any[] ? File[] : File 
			: U
		: never
}, 'id'>

// Create type for Auth collections
export type CreateAuth<T> = {
	id?: RecordIdString
	email: string
	emailVisibility?: boolean
	password: string
	passwordConfirm: string
	verified?: boolean
} & ProcessCreateAndUpdateFields<T>

// Create type for Base collections
export type CreateBase<T> = {
	id?: RecordIdString
} & ProcessCreateAndUpdateFields<T>

// Update type for Auth collections
export type UpdateAuth<T> = Partial<
	Omit<ProcessCreateAndUpdateFields<T>, keyof AuthSystemFields>
> & {
	email?: string
	emailVisibility?: boolean
	oldPassword?: string
	password?: string
	passwordConfirm?: string
	verified?: boolean
}

// Update type for Base collections
export type UpdateBase<T> = Partial<
	Omit<ProcessCreateAndUpdateFields<T>, keyof BaseSystemFields>
>

// Get the correct create type for any collection
export type Create<T extends keyof CollectionResponses> =
	CollectionResponses[T] extends AuthSystemFields
		? CreateAuth<CollectionRecords[T]>
		: CreateBase<CollectionRecords[T]>

// Get the correct update type for any collection
export type Update<T extends keyof CollectionResponses> =
	CollectionResponses[T] extends AuthSystemFields
		? UpdateAuth<CollectionRecords[T]>
		: UpdateBase<CollectionRecords[T]>

// Type for usage with type asserted PocketBase instance
// https://github.com/pocketbase/js-sdk#specify-typescript-definitions

export type TypedPocketBase = {
	collection<T extends keyof CollectionResponses>(
		idOrName: T
	): RecordService<CollectionResponses[T]>
} & PocketBase
