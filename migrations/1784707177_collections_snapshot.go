package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		jsonData := `[
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text455797646",
						"max": 0,
						"min": 0,
						"name": "collectionRef",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text127846527",
						"max": 0,
						"min": 0,
						"name": "recordRef",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1582905952",
						"max": 0,
						"min": 0,
						"name": "method",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": true,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": true,
						"type": "autodate"
					}
				],
				"id": "pbc_2279338944",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_mfas_collectionRef_recordRef` + "`" + ` ON ` + "`" + `_mfas` + "`" + ` (collectionRef,recordRef)"
				],
				"listRule": "@request.auth.id != '' && recordRef = @request.auth.id && collectionRef = @request.auth.collectionId",
				"name": "_mfas",
				"system": true,
				"type": "base",
				"updateRule": null,
				"viewRule": "@request.auth.id != '' && recordRef = @request.auth.id && collectionRef = @request.auth.collectionId"
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text455797646",
						"max": 0,
						"min": 0,
						"name": "collectionRef",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text127846527",
						"max": 0,
						"min": 0,
						"name": "recordRef",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cost": 8,
						"hidden": true,
						"id": "password901924565",
						"max": 0,
						"min": 0,
						"name": "password",
						"pattern": "",
						"presentable": false,
						"required": true,
						"system": true,
						"type": "password"
					},
					{
						"autogeneratePattern": "",
						"hidden": true,
						"id": "text3866985172",
						"max": 0,
						"min": 0,
						"name": "sentTo",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": true,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": true,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": true,
						"type": "autodate"
					}
				],
				"id": "pbc_1638494021",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_otps_collectionRef_recordRef` + "`" + ` ON ` + "`" + `_otps` + "`" + ` (collectionRef, recordRef)"
				],
				"listRule": "@request.auth.id != '' && recordRef = @request.auth.id && collectionRef = @request.auth.collectionId",
				"name": "_otps",
				"system": true,
				"type": "base",
				"updateRule": null,
				"viewRule": "@request.auth.id != '' && recordRef = @request.auth.id && collectionRef = @request.auth.collectionId"
			},
			{
				"createRule": null,
				"deleteRule": "@request.auth.id != '' && recordRef = @request.auth.id && collectionRef = @request.auth.collectionId",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text455797646",
						"max": 0,
						"min": 0,
						"name": "collectionRef",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text127846527",
						"max": 0,
						"min": 0,
						"name": "recordRef",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text2462348188",
						"max": 0,
						"min": 0,
						"name": "provider",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1044722854",
						"max": 0,
						"min": 0,
						"name": "providerId",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": true,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": true,
						"type": "autodate"
					}
				],
				"id": "pbc_2281828961",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_externalAuths_record_provider` + "`" + ` ON ` + "`" + `_externalAuths` + "`" + ` (collectionRef, recordRef, provider)",
					"CREATE UNIQUE INDEX ` + "`" + `idx_externalAuths_collection_provider` + "`" + ` ON ` + "`" + `_externalAuths` + "`" + ` (collectionRef, provider, providerId)"
				],
				"listRule": "@request.auth.id != '' && recordRef = @request.auth.id && collectionRef = @request.auth.collectionId",
				"name": "_externalAuths",
				"system": true,
				"type": "base",
				"updateRule": null,
				"viewRule": "@request.auth.id != '' && recordRef = @request.auth.id && collectionRef = @request.auth.collectionId"
			},
			{
				"createRule": null,
				"deleteRule": "@request.auth.id != '' && recordRef = @request.auth.id && collectionRef = @request.auth.collectionId",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text455797646",
						"max": 0,
						"min": 0,
						"name": "collectionRef",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text127846527",
						"max": 0,
						"min": 0,
						"name": "recordRef",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text4228609354",
						"max": 0,
						"min": 0,
						"name": "fingerprint",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": true,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": true,
						"type": "autodate"
					}
				],
				"id": "pbc_4275539003",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_authOrigins_unique_pairs` + "`" + ` ON ` + "`" + `_authOrigins` + "`" + ` (collectionRef, recordRef, fingerprint)"
				],
				"listRule": "@request.auth.id != '' && recordRef = @request.auth.id && collectionRef = @request.auth.collectionId",
				"name": "_authOrigins",
				"system": true,
				"type": "base",
				"updateRule": null,
				"viewRule": "@request.auth.id != '' && recordRef = @request.auth.id && collectionRef = @request.auth.collectionId"
			},
			{
				"authAlert": {
					"emailTemplate": {
						"body": "<p>Hello,</p>\n<p>We noticed a login to your {APP_NAME} account from a new location:</p>\n<p><em>{ALERT_INFO}</em></p>\n<p><strong>If this wasn't you, you should immediately change your {APP_NAME} account password to revoke access from all other locations.</strong></p>\n<p>If this was you, you may disregard this email.</p>\n<p>\n  Thanks,<br/>\n  {APP_NAME} team\n</p>",
						"subject": "Login from a new location"
					},
					"enabled": true
				},
				"authRule": "",
				"authToken": {
					"duration": 86400
				},
				"confirmEmailChangeTemplate": {
					"body": "<p>Hello,</p>\n<p>Click on the button below to confirm your new email address.</p>\n<p>\n  <a class=\"btn\" href=\"{APP_URL}/_/#/auth/confirm-email-change/{TOKEN}\" target=\"_blank\" rel=\"noopener\">Confirm new email</a>\n</p>\n<p><i>If you didn't ask to change your email address, you can ignore this email.</i></p>\n<p>\n  Thanks,<br/>\n  {APP_NAME} team\n</p>",
					"subject": "Confirm your {APP_NAME} new email address"
				},
				"createRule": null,
				"deleteRule": null,
				"emailChangeToken": {
					"duration": 1800
				},
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cost": 0,
						"hidden": true,
						"id": "password901924565",
						"max": 0,
						"min": 8,
						"name": "password",
						"pattern": "",
						"presentable": false,
						"required": true,
						"system": true,
						"type": "password"
					},
					{
						"autogeneratePattern": "[a-zA-Z0-9]{50}",
						"hidden": true,
						"id": "text2504183744",
						"max": 60,
						"min": 30,
						"name": "tokenKey",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"exceptDomains": null,
						"hidden": false,
						"id": "email3885137012",
						"name": "email",
						"onlyDomains": null,
						"presentable": false,
						"required": true,
						"system": true,
						"type": "email"
					},
					{
						"hidden": false,
						"id": "bool1547992806",
						"name": "emailVisibility",
						"presentable": false,
						"required": false,
						"system": true,
						"type": "bool"
					},
					{
						"hidden": false,
						"id": "bool256245529",
						"name": "verified",
						"presentable": false,
						"required": false,
						"system": true,
						"type": "bool"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": true,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": true,
						"type": "autodate"
					}
				],
				"fileToken": {
					"duration": 180
				},
				"id": "pbc_3142635823",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_tokenKey_pbc_3142635823` + "`" + ` ON ` + "`" + `_superusers` + "`" + ` (` + "`" + `tokenKey` + "`" + `)",
					"CREATE UNIQUE INDEX ` + "`" + `idx_email_pbc_3142635823` + "`" + ` ON ` + "`" + `_superusers` + "`" + ` (` + "`" + `email` + "`" + `) WHERE ` + "`" + `email` + "`" + ` != ''"
				],
				"listRule": null,
				"manageRule": null,
				"mfa": {
					"duration": 1800,
					"enabled": false,
					"rule": ""
				},
				"name": "_superusers",
				"oauth2": {
					"enabled": false,
					"mappedFields": {
						"avatarURL": "",
						"id": "",
						"name": "",
						"username": ""
					}
				},
				"otp": {
					"duration": 180,
					"emailTemplate": {
						"body": "<p>Hello,</p>\n<p>Your one-time password is: <strong>{OTP}</strong></p>\n<p><i>If you didn't ask for the one-time password, you can ignore this email.</i></p>\n<p>\n  Thanks,<br/>\n  {APP_NAME} team\n</p>",
						"subject": "OTP for {APP_NAME}"
					},
					"enabled": false,
					"length": 8
				},
				"passwordAuth": {
					"enabled": true,
					"identityFields": [
						"email"
					]
				},
				"passwordResetToken": {
					"duration": 1800
				},
				"resetPasswordTemplate": {
					"body": "<p>Hello,</p>\n<p>Click on the button below to reset your password.</p>\n<p>\n  <a class=\"btn\" href=\"{APP_URL}/_/#/auth/confirm-password-reset/{TOKEN}\" target=\"_blank\" rel=\"noopener\">Reset password</a>\n</p>\n<p><i>If you didn't ask to reset your password, you can ignore this email.</i></p>\n<p>\n  Thanks,<br/>\n  {APP_NAME} team\n</p>",
					"subject": "Reset your {APP_NAME} password"
				},
				"system": true,
				"type": "auth",
				"updateRule": null,
				"verificationTemplate": {
					"body": "<p>Hello,</p>\n<p>Thank you for joining us at {APP_NAME}.</p>\n<p>Click on the button below to verify your email address.</p>\n<p>\n  <a class=\"btn\" href=\"{APP_URL}/_/#/auth/confirm-verification/{TOKEN}\" target=\"_blank\" rel=\"noopener\">Verify</a>\n</p>\n<p>\n  Thanks,<br/>\n  {APP_NAME} team\n</p>",
					"subject": "Verify your {APP_NAME} email"
				},
				"verificationToken": {
					"duration": 259200
				},
				"viewRule": null
			},
			{
				"authAlert": {
					"emailTemplate": {
						"body": "<p>Hello,</p>\n<p>We noticed a login to your {APP_NAME} account from a new location:</p>\n<p><em>{ALERT_INFO}</em></p>\n<p><strong>If this wasn't you, you should immediately change your {APP_NAME} account password to revoke access from all other locations.</strong></p>\n<p>If this was you, you may disregard this email.</p>\n<p>\n  Thanks,<br/>\n  {APP_NAME} team\n</p>",
						"subject": "Login from a new location"
					},
					"enabled": true
				},
				"authRule": "is_deleted = false && (is_banned = false || (banned_until != \"\" && banned_until < @now))",
				"authToken": {
					"duration": 604800
				},
				"confirmEmailChangeTemplate": {
					"body": "<p>Hello,</p>\n<p>Click on the button below to confirm your new email address.</p>\n<p>\n  <a class=\"btn\" href=\"{APP_URL}/_/#/auth/confirm-email-change/{TOKEN}\" target=\"_blank\" rel=\"noopener\">Confirm new email</a>\n</p>\n<p><i>If you didn't ask to change your email address, you can ignore this email.</i></p>\n<p>\n  Thanks,<br/>\n  {APP_NAME} team\n</p>",
					"subject": "Confirm your {APP_NAME} new email address"
				},
				"createRule": "",
				"deleteRule": "id = @request.auth.id",
				"emailChangeToken": {
					"duration": 1800
				},
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cost": 0,
						"hidden": true,
						"id": "password901924565",
						"max": 0,
						"min": 8,
						"name": "password",
						"pattern": "",
						"presentable": false,
						"required": true,
						"system": true,
						"type": "password"
					},
					{
						"autogeneratePattern": "[a-zA-Z0-9]{50}",
						"hidden": true,
						"id": "text2504183744",
						"max": 60,
						"min": 30,
						"name": "tokenKey",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"exceptDomains": null,
						"hidden": false,
						"id": "email3885137012",
						"name": "email",
						"onlyDomains": null,
						"presentable": false,
						"required": true,
						"system": true,
						"type": "email"
					},
					{
						"hidden": false,
						"id": "bool1547992806",
						"name": "emailVisibility",
						"presentable": false,
						"required": false,
						"system": true,
						"type": "bool"
					},
					{
						"hidden": false,
						"id": "bool256245529",
						"name": "verified",
						"presentable": false,
						"required": false,
						"system": true,
						"type": "bool"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1579384326",
						"max": 255,
						"min": 0,
						"name": "name",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "file376926767",
						"maxSelect": 1,
						"maxSize": 0,
						"mimeTypes": [
							"image/jpeg",
							"image/png",
							"image/svg+xml",
							"image/gif",
							"image/webp"
						],
						"name": "avatar",
						"presentable": false,
						"protected": false,
						"required": false,
						"system": false,
						"thumbs": null,
						"type": "file"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text4166911607",
						"max": 34,
						"min": 2,
						"name": "username",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1250677669",
						"max": 11,
						"min": 0,
						"name": "gamertag",
						"pattern": "^[ -~]*$",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text3709889147",
						"max": 500,
						"min": 0,
						"name": "bio",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1587448267",
						"max": 100,
						"min": 0,
						"name": "location",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "bool4245145851",
						"name": "is_deleted",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					},
					{
						"hidden": false,
						"id": "date1257476049",
						"max": "",
						"min": "",
						"name": "deleted_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "bool3405807721",
						"name": "is_banned",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					},
					{
						"hidden": false,
						"id": "date642497888",
						"max": "",
						"min": "",
						"name": "banned_until",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_3654546727",
						"hidden": false,
						"id": "relation1288692715",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "default_gamertag",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					}
				],
				"fileToken": {
					"duration": 180
				},
				"id": "_pb_users_auth_",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_tokenKey__pb_users_auth_` + "`" + ` ON ` + "`" + `users` + "`" + ` (` + "`" + `tokenKey` + "`" + `)",
					"CREATE UNIQUE INDEX ` + "`" + `idx_email__pb_users_auth_` + "`" + ` ON ` + "`" + `users` + "`" + ` (` + "`" + `email` + "`" + `) WHERE ` + "`" + `email` + "`" + ` != ''",
					"CREATE UNIQUE INDEX ` + "`" + `idx_users_username_unique` + "`" + ` ON ` + "`" + `users` + "`" + ` (username)"
				],
				"listRule": "id = @request.auth.id || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"manageRule": null,
				"mfa": {
					"duration": 1800,
					"enabled": false,
					"rule": ""
				},
				"name": "users",
				"oauth2": {
					"enabled": false,
					"mappedFields": {
						"avatarURL": "avatar",
						"id": "",
						"name": "",
						"username": "username"
					}
				},
				"otp": {
					"duration": 180,
					"emailTemplate": {
						"body": "<p>Hello,</p>\n<p>Your one-time password is: <strong>{OTP}</strong></p>\n<p><i>If you didn't ask for the one-time password, you can ignore this email.</i></p>\n<p>\n  Thanks,<br/>\n  {APP_NAME} team\n</p>",
						"subject": "OTP for {APP_NAME}"
					},
					"enabled": false,
					"length": 8
				},
				"passwordAuth": {
					"enabled": true,
					"identityFields": [
						"email"
					]
				},
				"passwordResetToken": {
					"duration": 1800
				},
				"resetPasswordTemplate": {
					"body": "<p>Hello,</p>\n<p>Click on the button below to reset your password.</p>\n<p>\n  <a class=\"btn\" href=\"{APP_URL}/_/#/auth/confirm-password-reset/{TOKEN}\" target=\"_blank\" rel=\"noopener\">Reset password</a>\n</p>\n<p><i>If you didn't ask to reset your password, you can ignore this email.</i></p>\n<p>\n  Thanks,<br/>\n  {APP_NAME} team\n</p>",
					"subject": "Reset your {APP_NAME} password"
				},
				"system": false,
				"type": "auth",
				"updateRule": "id = @request.auth.id",
				"verificationTemplate": {
					"body": "<p>Hello,</p>\n<p>Thank you for joining us at {APP_NAME}.</p>\n<p>Click on the button below to verify your email address.</p>\n<p>\n  <a class=\"btn\" href=\"{APP_URL}/_/#/auth/confirm-verification/{TOKEN}\" target=\"_blank\" rel=\"noopener\">Verify</a>\n</p>\n<p>\n  Thanks,<br/>\n  {APP_NAME} team\n</p>",
					"subject": "Verify your {APP_NAME} email"
				},
				"verificationToken": {
					"duration": 259200
				},
				"viewRule": "id = @request.auth.id || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")"
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1579384326",
						"max": 64,
						"min": 1,
						"name": "name",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text3578368839",
						"max": 15,
						"min": 0,
						"name": "display_name",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "number2155046657",
						"max": null,
						"min": 0,
						"name": "index",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "number2290671711",
						"max": null,
						"min": 0,
						"name": "xemu_http",
						"onlyInt": true,
						"presentable": false,
						"required": true,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "number3763473294",
						"max": null,
						"min": 0,
						"name": "xemu_https",
						"onlyInt": true,
						"presentable": false,
						"required": true,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "number3909327539",
						"max": null,
						"min": 0,
						"name": "xemu_ws",
						"onlyInt": true,
						"presentable": false,
						"required": true,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "number3414230027",
						"max": null,
						"min": 0,
						"name": "browser_web",
						"onlyInt": true,
						"presentable": false,
						"required": true,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "number1588627297",
						"max": null,
						"min": 0,
						"name": "browser_vnc",
						"onlyInt": true,
						"presentable": false,
						"required": true,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "date2990389176",
						"max": "",
						"min": "",
						"name": "created",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"autogeneratePattern": "",
						"hidden": true,
						"id": "text1977349933",
						"max": 128,
						"min": 0,
						"name": "vnc_password",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "bool3993433790",
						"name": "is_neutral_host",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					}
				],
				"id": "pbc_1864144027",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_containers_name_unique` + "`" + ` ON ` + "`" + `containers` + "`" + ` (name)",
					"CREATE UNIQUE INDEX ` + "`" + `idx_containers_index_unique` + "`" + ` ON ` + "`" + `containers` + "`" + ` (index)"
				],
				"listRule": null,
				"name": "containers",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": null
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1596010991",
						"max": 32,
						"min": 1,
						"name": "guild_id",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "select4104478782",
						"maxSelect": 4,
						"name": "posted_categories",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "select",
						"values": [
							"casual",
							"competitive",
							"tournament",
							"custom"
						]
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_2445088841",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_discord_guild_settings_guild_id_unique` + "`" + ` ON ` + "`" + `discord_guild_settings` + "`" + ` (guild_id)"
				],
				"listRule": null,
				"name": "discord_guild_settings",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": null
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1596010991",
						"max": 32,
						"min": 1,
						"name": "guild_id",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text2757247829",
						"max": 64,
						"min": 1,
						"name": "hook",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1928700330",
						"max": 32,
						"min": 1,
						"name": "channel_id",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_2352455496",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_discord_routes_guild_hook_unique` + "`" + ` ON ` + "`" + `discord_routes` + "`" + ` (guild_id, hook)",
					"CREATE INDEX ` + "`" + `idx_discord_routes_guild` + "`" + ` ON ` + "`" + `discord_routes` + "`" + ` (guild_id)"
				],
				"listRule": null,
				"name": "discord_routes",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": null
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text2560465762",
						"max": 60,
						"min": 2,
						"name": "slug",
						"pattern": "^[a-z0-9]+(?:_[a-z0-9]+)*$",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text245846248",
						"max": 80,
						"min": 1,
						"name": "label",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1843675174",
						"max": 500,
						"min": 0,
						"name": "description",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "number2599078931",
						"max": 100,
						"min": 0,
						"name": "level",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_2105053228",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_roles_slug_unique` + "`" + ` ON ` + "`" + `roles` + "`" + ` (slug)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "roles",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation2375276105",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "user",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_2105053228",
						"hidden": false,
						"id": "relation1466534506",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "role",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation2784720191",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "granted_by",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "autodate4033305153",
						"name": "granted_at",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_2857166095",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_user_roles_user_role_unique` + "`" + ` ON ` + "`" + `user_roles` + "`" + ` (user, role)",
					"CREATE INDEX ` + "`" + `idx_user_roles_user` + "`" + ` ON ` + "`" + `user_roles` + "`" + ` (user)"
				],
				"listRule": "user = @request.auth.id",
				"name": "user_roles",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": "user = @request.auth.id"
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation1148540665",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "actor",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text3466706339",
						"max": 0,
						"min": 1,
						"name": "target_collection",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text361630566",
						"max": 0,
						"min": 1,
						"name": "target_id",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1204587666",
						"max": 0,
						"min": 1,
						"name": "action",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "json966904008",
						"maxSize": 524288,
						"name": "payload_json",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_2462721645",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_audit_log_target_created` + "`" + ` ON ` + "`" + `audit_log` + "`" + ` (target_collection, target_id, created DESC)",
					"CREATE INDEX ` + "`" + `idx_audit_log_actor_created` + "`" + ` ON ` + "`" + `audit_log` + "`" + ` (actor, created DESC)",
					"CREATE INDEX ` + "`" + `idx_audit_log_action_created` + "`" + ` ON ` + "`" + `audit_log` + "`" + ` (action, created DESC)"
				],
				"listRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"",
				"name": "audit_log",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\""
			},
			{
				"createRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"",
				"deleteRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1110487518",
						"max": 64,
						"min": 1,
						"name": "instance",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text3981121951",
						"max": 32,
						"min": 1,
						"name": "class",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "select2546616235",
						"maxSelect": 1,
						"name": "mode",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "select",
						"values": [
							"auto",
							"always",
							"never"
						]
					},
					{
						"hidden": false,
						"id": "select3046230236",
						"maxSelect": 1,
						"name": "cadence",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "select",
						"values": [
							"default",
							"engine",
							"30hz",
							"10hz",
							"5hz",
							"250ms",
							"500ms",
							"1s"
						]
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1054268984",
						"max": 256,
						"min": 0,
						"name": "sink",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1843675174",
						"max": 256,
						"min": 0,
						"name": "description",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_2358364955",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_capture_policies_instance_class_unique` + "`" + ` ON ` + "`" + `capture_policies` + "`" + ` (instance, class)"
				],
				"listRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"",
				"name": "capture_policies",
				"system": false,
				"type": "base",
				"updateRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"",
				"viewRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\""
			},
			{
				"createRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"",
				"deleteRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1250677669",
						"max": 64,
						"min": 1,
						"name": "gamertag",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text3485334036",
						"max": 256,
						"min": 0,
						"name": "note",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_1991406265",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_dummy_gamertags_gamertag_unique` + "`" + ` ON ` + "`" + `dummy_gamertags` + "`" + ` (gamertag)"
				],
				"listRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"",
				"name": "dummy_gamertags",
				"system": false,
				"type": "base",
				"updateRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"",
				"viewRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\""
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation2375276105",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "user",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text2363381545",
						"max": 0,
						"min": 1,
						"name": "type",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "json966904008",
						"maxSize": 262144,
						"name": "payload_json",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"hidden": false,
						"id": "bool2555855207",
						"name": "read",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					},
					{
						"hidden": false,
						"id": "date3805952114",
						"max": "",
						"min": "",
						"name": "read_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_2301922722",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_notifications_user_read_created` + "`" + ` ON ` + "`" + `notifications` + "`" + ` (user, read, created DESC)",
					"CREATE INDEX ` + "`" + `idx_notifications_user_created` + "`" + ` ON ` + "`" + `notifications` + "`" + ` (user, created DESC)"
				],
				"listRule": "user = @request.auth.id || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"name": "notifications",
				"system": false,
				"type": "base",
				"updateRule": "user = @request.auth.id || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"viewRule": "user = @request.auth.id || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")"
			},
			{
				"createRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"",
				"deleteRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text2747071630",
						"max": 64,
						"min": 1,
						"name": "pattern",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1843675174",
						"max": 300,
						"min": 0,
						"name": "description",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation3725765462",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "created_by",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_3286521383",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_reserved_names_pattern_unique` + "`" + ` ON ` + "`" + `reserved_names` + "`" + ` (pattern)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "reserved_names",
				"system": false,
				"type": "base",
				"updateRule": "@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"",
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": "@request.auth.id = user.id || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"deleteRule": "(@request.auth.id = user.id && status != \"blocked\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation2375276105",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "user",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text59357059",
						"max": 12,
						"min": 1,
						"name": "tag",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text2254113554",
						"max": 12,
						"min": 1,
						"name": "sanitized",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "select2063623452",
						"maxSelect": 0,
						"name": "status",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "select",
						"values": [
							"approved",
							"allowed",
							"pending",
							"blocked"
						]
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_3654546727",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_gamertags_user_sanitized_unique` + "`" + ` ON ` + "`" + `gamertags` + "`" + ` (user, sanitized)",
					"CREATE INDEX ` + "`" + `idx_gamertags_sanitized` + "`" + ` ON ` + "`" + `gamertags` + "`" + ` (sanitized)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "gamertags",
				"system": false,
				"type": "base",
				"updateRule": "(@request.auth.id = user.id && status != \"blocked\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": "@request.auth.id != \"\"",
				"deleteRule": "(created_by = @request.auth.id && status != \"blocked\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1579384326",
						"max": 60,
						"min": 2,
						"name": "name",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text2560465762",
						"max": 60,
						"min": 2,
						"name": "slug",
						"pattern": "^[a-z0-9]+(?:-[a-z0-9]+)*$",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation3725765462",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "created_by",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "select2063623452",
						"maxSelect": 0,
						"name": "status",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "select",
						"values": [
							"approved",
							"allowed",
							"pending",
							"blocked"
						]
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_1568971955",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_teams_slug_unique` + "`" + ` ON ` + "`" + `teams` + "`" + ` (slug)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "teams",
				"system": false,
				"type": "base",
				"updateRule": "(created_by = @request.auth.id && status != \"blocked\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": "(@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\") || team.created_by = @request.auth.id",
				"deleteRule": "(@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\") || team.created_by = @request.auth.id",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_1568971955",
						"hidden": false,
						"id": "relation3303056927",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "team",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_3654546727",
						"hidden": false,
						"id": "relation1250677669",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "gamertag",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "bool2208040204",
						"name": "is_owner",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					},
					{
						"hidden": false,
						"id": "bool3961037681",
						"name": "is_manager",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					},
					{
						"hidden": false,
						"id": "date2745685176",
						"max": "",
						"min": "",
						"name": "joined_at",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "date922899566",
						"max": "",
						"min": "",
						"name": "left_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_1314669556",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_rosters_team_left_at` + "`" + ` ON ` + "`" + `rosters` + "`" + ` (team, left_at)",
					"CREATE INDEX ` + "`" + `idx_rosters_gamertag` + "`" + ` ON ` + "`" + `rosters` + "`" + ` (gamertag)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "rosters",
				"system": false,
				"type": "base",
				"updateRule": "(@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\") || team.created_by = @request.auth.id",
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_1568971955",
						"hidden": false,
						"id": "relation3303056927",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "team",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1001261735",
						"max": 0,
						"min": 1,
						"name": "event",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation1148540665",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "actor",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation525947538",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "subject_user",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_3654546727",
						"hidden": false,
						"id": "relation1928762317",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "subject_gamertag",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "json966904008",
						"maxSize": 262144,
						"name": "payload_json",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_2514664056",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_team_log_team_created` + "`" + ` ON ` + "`" + `team_log` + "`" + ` (team, created DESC)",
					"CREATE INDEX ` + "`" + `idx_team_log_team_event_created` + "`" + ` ON ` + "`" + `team_log` + "`" + ` (team, event, created DESC)",
					"CREATE INDEX ` + "`" + `idx_team_log_subject_user_created` + "`" + ` ON ` + "`" + `team_log` + "`" + ` (subject_user, created DESC)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "team_log",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_1568971955",
						"hidden": false,
						"id": "relation3303056927",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "team",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation2375276105",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "user",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "select1045090739",
						"maxSelect": 0,
						"name": "direction",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "select",
						"values": [
							"invited",
							"requested"
						]
					},
					{
						"hidden": false,
						"id": "select2063623452",
						"maxSelect": 0,
						"name": "status",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "select",
						"values": [
							"pending",
							"accepted",
							"declined",
							"expired",
							"cancelled"
						]
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation3263369956",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "initiated_by",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation3266963338",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "responded_by",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "date261981154",
						"max": "",
						"min": "",
						"name": "expires_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_915259691",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_tmr_team_user_status` + "`" + ` ON ` + "`" + `team_membership_requests` + "`" + ` (team, user, status)",
					"CREATE INDEX ` + "`" + `idx_tmr_user_status_created` + "`" + ` ON ` + "`" + `team_membership_requests` + "`" + ` (user, status, created DESC)"
				],
				"listRule": "user = @request.auth.id || initiated_by = @request.auth.id || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"name": "team_membership_requests",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": "user = @request.auth.id || initiated_by = @request.auth.id || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")"
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1579384326",
						"max": 128,
						"min": 0,
						"name": "name",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "select3736761055",
						"maxSelect": 1,
						"name": "format",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "select",
						"values": [
							"single",
							"exact-n",
							"best-of-n",
							"first-to-x"
						]
					},
					{
						"hidden": false,
						"id": "number3307784892",
						"max": null,
						"min": 1,
						"name": "target_n",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "select105650625",
						"maxSelect": 1,
						"name": "category",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "select",
						"values": [
							"casual",
							"competitive",
							"tournament",
							"custom"
						]
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation3725765462",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "created_by",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "date222754019",
						"max": "",
						"min": "",
						"name": "started_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "date473765221",
						"max": "",
						"min": "",
						"name": "ended_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text3177167065",
						"max": 64,
						"min": 0,
						"name": "tournament",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "number79198765",
						"max": null,
						"min": 0,
						"name": "tournament_round",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_218332259",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_series_category_created` + "`" + ` ON ` + "`" + `series` + "`" + ` (category, created)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "series",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_218332259",
						"hidden": false,
						"id": "relation974127405",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "series",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text3349343259",
						"max": 64,
						"min": 0,
						"name": "container",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1381318223",
						"max": 64,
						"min": 0,
						"name": "host_machine_name",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text2477632187",
						"max": 64,
						"min": 0,
						"name": "map",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text3673120022",
						"max": 64,
						"min": 0,
						"name": "gametype",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text2983979155",
						"max": 128,
						"min": 0,
						"name": "variant_name",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "date222754019",
						"max": "",
						"min": "",
						"name": "started_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "date473765221",
						"max": "",
						"min": "",
						"name": "ended_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "number3137288",
						"max": null,
						"min": null,
						"name": "winner_team",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text2946515959",
						"max": 256,
						"min": 0,
						"name": "score_summary",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "json2721001649",
						"maxSize": 1048576,
						"name": "snapshot_blob",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_879072730",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_games_series` + "`" + ` ON ` + "`" + `games` + "`" + ` (series)",
					"CREATE INDEX ` + "`" + `idx_games_ended_at` + "`" + ` ON ` + "`" + `games` + "`" + ` (ended_at)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "games",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_879072730",
						"hidden": false,
						"id": "relation590033292",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "game",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1250677669",
						"max": 64,
						"min": 0,
						"name": "gamertag",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "number3303056927",
						"max": null,
						"min": null,
						"name": "team",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "number795295649",
						"max": null,
						"min": null,
						"name": "kills",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "number3342828817",
						"max": null,
						"min": null,
						"name": "deaths",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "number296718251",
						"max": null,
						"min": null,
						"name": "assists",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "number848901969",
						"max": null,
						"min": null,
						"name": "score",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "number3135113788",
						"max": null,
						"min": 0,
						"name": "time_alive_ms",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "json3679620614",
						"maxSize": 65536,
						"name": "weapon_loadout",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_3826546831",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_game_players_game` + "`" + ` ON ` + "`" + `game_players` + "`" + ` (game)",
					"CREATE INDEX ` + "`" + `idx_game_players_gamertag` + "`" + ` ON ` + "`" + `game_players` + "`" + ` (gamertag)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "game_players",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1110487518",
						"max": 64,
						"min": 1,
						"name": "instance",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text2363381545",
						"max": 32,
						"min": 1,
						"name": "type",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "number2524893523",
						"max": null,
						"min": 0,
						"name": "seq",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "number380606668",
						"max": null,
						"min": 0,
						"name": "tick",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "date3280375435",
						"max": "",
						"min": "",
						"name": "ts",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "json2918445923",
						"maxSize": 1048576,
						"name": "data",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_879072730",
						"hidden": false,
						"id": "relation590033292",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "game",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_3728988662",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_game_events_instance_seq` + "`" + ` ON ` + "`" + `game_events` + "`" + ` (instance, seq)",
					"CREATE INDEX ` + "`" + `idx_game_events_instance_type` + "`" + ` ON ` + "`" + `game_events` + "`" + ` (instance, type)",
					"CREATE INDEX ` + "`" + `idx_game_events_game` + "`" + ` ON ` + "`" + `game_events` + "`" + ` (game)"
				],
				"listRule": null,
				"name": "game_events",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": null
			},
			{
				"createRule": "(@request.auth.id = user) || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"deleteRule": "(@request.auth.id = user) || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cascadeDelete": true,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation2375276105",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "user",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "json3846545605",
						"maxSize": 65536,
						"name": "settings",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"hidden": false,
						"id": "file3908367568",
						"maxSelect": 1,
						"maxSize": 1048576,
						"mimeTypes": null,
						"name": "save_bundle",
						"presentable": false,
						"protected": false,
						"required": false,
						"system": false,
						"thumbs": null,
						"type": "file"
					},
					{
						"hidden": false,
						"id": "json3026814514",
						"maxSize": 65536,
						"name": "save_info",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_645364075",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_ce_profiles_user_unique` + "`" + ` ON ` + "`" + `ce_profiles` + "`" + ` (user)"
				],
				"listRule": "(@request.auth.id = user) || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"name": "ce_profiles",
				"system": false,
				"type": "base",
				"updateRule": "(@request.auth.id = user) || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"viewRule": "(@request.auth.id = user) || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")"
			},
			{
				"createRule": "(@request.auth.id = user) || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"deleteRule": "(@request.auth.id = user) || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cascadeDelete": true,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation2375276105",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "user",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "json2863891088",
						"maxSize": 65536,
						"name": "appearance",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"hidden": false,
						"id": "file3908367568",
						"maxSelect": 1,
						"maxSize": 1048576,
						"mimeTypes": null,
						"name": "save_bundle",
						"presentable": false,
						"protected": false,
						"required": false,
						"system": false,
						"thumbs": null,
						"type": "file"
					},
					{
						"hidden": false,
						"id": "json3026814514",
						"maxSize": 65536,
						"name": "save_info",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_1587438366",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_h2_profiles_user_unique` + "`" + ` ON ` + "`" + `h2_profiles` + "`" + ` (user)"
				],
				"listRule": "(@request.auth.id = user) || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"name": "h2_profiles",
				"system": false,
				"type": "base",
				"updateRule": "(@request.auth.id = user) || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")",
				"viewRule": "(@request.auth.id = user) || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")"
			},
			{
				"createRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"deleteRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "select724990059",
						"maxSelect": 1,
						"name": "title",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "select",
						"values": [
							"ce",
							"h2"
						]
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text3903330957",
						"max": 32,
						"min": 1,
						"name": "engine",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1579384326",
						"max": 64,
						"min": 1,
						"name": "name",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "json3846545605",
						"maxSize": 65536,
						"name": "settings",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"hidden": false,
						"id": "file3908367568",
						"maxSelect": 1,
						"maxSize": 1048576,
						"mimeTypes": null,
						"name": "save_bundle",
						"presentable": false,
						"protected": false,
						"required": false,
						"system": false,
						"thumbs": null,
						"type": "file"
					},
					{
						"hidden": false,
						"id": "json3026814514",
						"maxSize": 65536,
						"name": "save_info",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation3725765462",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "created_by",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_2334880671",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_gametypes_title` + "`" + ` ON ` + "`" + `gametypes` + "`" + ` (title)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "gametypes",
				"system": false,
				"type": "base",
				"updateRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"deleteRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1579384326",
						"max": 120,
						"min": 1,
						"name": "name",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1843675174",
						"max": 2000,
						"min": 0,
						"name": "description",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "file2359244304",
						"maxSelect": 1,
						"maxSize": 2147483648,
						"mimeTypes": null,
						"name": "file",
						"presentable": false,
						"protected": false,
						"required": true,
						"system": false,
						"thumbs": null,
						"type": "file"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation3725765462",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "created_by",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_1283575583",
				"indexes": [],
				"listRule": "@request.auth.id != \"\"",
				"name": "game_titles",
				"system": false,
				"type": "base",
				"updateRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1579384326",
						"max": 120,
						"min": 1,
						"name": "name",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1007413605",
						"max": 255,
						"min": 1,
						"name": "filename",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text178227133",
						"max": 32,
						"min": 0,
						"name": "title_id",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1843675174",
						"max": 2000,
						"min": 0,
						"name": "description",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "bool2777654405",
						"name": "available",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text4154070712",
						"max": 64,
						"min": 0,
						"name": "dest_name",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text291047825",
						"max": 1024,
						"min": 0,
						"name": "extracted_path",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "bool1060648558",
						"name": "extracted_ready",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					},
					{
						"hidden": false,
						"id": "date828770269",
						"max": "",
						"min": "",
						"name": "extracted_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "number3370804256",
						"max": null,
						"min": 0,
						"name": "footprint_bytes",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_1035567554",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_isos_filename_unique` + "`" + ` ON ` + "`" + `isos` + "`" + ` (filename)"
				],
				"listRule": null,
				"name": "isos",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": null
			},
			{
				"createRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"deleteRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1579384326",
						"max": 120,
						"min": 1,
						"name": "name",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1843675174",
						"max": 2000,
						"min": 0,
						"name": "description",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text178227133",
						"max": 32,
						"min": 0,
						"name": "title_id",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "file2359244304",
						"maxSelect": 1,
						"maxSize": 536870912,
						"mimeTypes": null,
						"name": "file",
						"presentable": false,
						"protected": false,
						"required": true,
						"system": false,
						"thumbs": null,
						"type": "file"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text4154070712",
						"max": 64,
						"min": 0,
						"name": "dest_name",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text291047825",
						"max": 1024,
						"min": 0,
						"name": "extracted_path",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "bool1060648558",
						"name": "extracted_ready",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					},
					{
						"hidden": false,
						"id": "date828770269",
						"max": "",
						"min": "",
						"name": "extracted_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "number3370804256",
						"max": null,
						"min": 0,
						"name": "footprint_bytes",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "bool2777654405",
						"name": "available",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation3725765462",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "created_by",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_923867626",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_apps_name` + "`" + ` ON ` + "`" + `apps` + "`" + ` (name)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "apps",
				"system": false,
				"type": "base",
				"updateRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"deleteRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text245846248",
						"max": 120,
						"min": 1,
						"name": "label",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1843675174",
						"max": 2000,
						"min": 0,
						"name": "description",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "date1436569724",
						"max": "",
						"min": "",
						"name": "starts_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "bool1260321794",
						"name": "active",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_1866527937",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_lan_events_active` + "`" + ` ON ` + "`" + `lan_events` + "`" + ` (active)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "lan_events",
				"system": false,
				"type": "base",
				"updateRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": "(@request.auth.id = gamertag.user || ((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")))",
				"deleteRule": "(@request.auth.id = gamertag.user || ((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")))",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"cascadeDelete": true,
						"collectionId": "pbc_1866527937",
						"hidden": false,
						"id": "relation1001261735",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "event",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"cascadeDelete": true,
						"collectionId": "pbc_3654546727",
						"hidden": false,
						"id": "relation1250677669",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "gamertag",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "select1602912115",
						"maxSelect": 1,
						"name": "source",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "select",
						"values": [
							"self",
							"organizer"
						]
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_3702085596",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_checkins_event_gamertag` + "`" + ` ON ` + "`" + `checkins` + "`" + ` (event, gamertag)",
					"CREATE INDEX ` + "`" + `idx_checkins_event` + "`" + ` ON ` + "`" + `checkins` + "`" + ` (event)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "checkins",
				"system": false,
				"type": "base",
				"updateRule": "(@request.auth.id = gamertag.user || ((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\")))",
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"deleteRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1579384326",
						"max": 120,
						"min": 1,
						"name": "name",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "bool1260321794",
						"name": "active",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_1866527937",
						"hidden": false,
						"id": "relation1001261735",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "event",
						"presentable": false,
						"required": true,
						"system": false,
						"type": "relation"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_1035567554",
						"hidden": false,
						"id": "relation4280494897",
						"maxSelect": 999,
						"minSelect": 0,
						"name": "games",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"cascadeDelete": false,
						"collectionId": "pbc_923867626",
						"hidden": false,
						"id": "relation270302810",
						"maxSelect": 999,
						"minSelect": 0,
						"name": "apps",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "json1655102503",
						"maxSize": 65536,
						"name": "priority",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"hidden": false,
						"id": "json4034725142",
						"maxSize": 65536,
						"name": "policy",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "json"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_2460601282",
				"indexes": [
					"CREATE INDEX ` + "`" + `idx_sync_presets_active` + "`" + ` ON ` + "`" + `sync_presets` + "`" + ` (active)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "sync_presets",
				"system": false,
				"type": "base",
				"updateRule": "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))",
				"viewRule": "@request.auth.id != \"\""
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1159956604",
						"max": 64,
						"min": 1,
						"name": "kid",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1923043739",
						"max": 96,
						"min": 1,
						"name": "room",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text11490771",
						"max": 32,
						"min": 0,
						"name": "scope",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text245846248",
						"max": 128,
						"min": 0,
						"name": "label",
						"pattern": "",
						"presentable": false,
						"primaryKey": false,
						"required": false,
						"system": false,
						"type": "text"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation870285563",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "minted_by",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "date261981154",
						"max": "",
						"min": "",
						"name": "expires_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"hidden": false,
						"id": "bool3181538509",
						"name": "revoked",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "bool"
					},
					{
						"hidden": false,
						"id": "date3687365789",
						"max": "",
						"min": "",
						"name": "revoked_at",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "date"
					},
					{
						"cascadeDelete": false,
						"collectionId": "_pb_users_auth_",
						"hidden": false,
						"id": "relation2387907555",
						"maxSelect": 1,
						"minSelect": 0,
						"name": "revoked_by",
						"presentable": false,
						"required": false,
						"system": false,
						"type": "relation"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_3715020333",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_overlay_tokens_kid_unique` + "`" + ` ON ` + "`" + `overlay_tokens` + "`" + ` (kid)",
					"CREATE INDEX ` + "`" + `idx_overlay_tokens_room` + "`" + ` ON ` + "`" + `overlay_tokens` + "`" + ` (room)"
				],
				"listRule": null,
				"name": "overlay_tokens",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": null
			},
			{
				"createRule": null,
				"deleteRule": null,
				"fields": [
					{
						"autogeneratePattern": "[a-z0-9]{15}",
						"hidden": false,
						"id": "text3208210256",
						"max": 15,
						"min": 15,
						"name": "id",
						"pattern": "^[a-z0-9]+$",
						"presentable": false,
						"primaryKey": true,
						"required": true,
						"system": true,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text1250677669",
						"max": 64,
						"min": 0,
						"name": "gamertag",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"autogeneratePattern": "",
						"hidden": false,
						"id": "text3673120022",
						"max": 64,
						"min": 0,
						"name": "gametype",
						"pattern": "",
						"presentable": true,
						"primaryKey": false,
						"required": true,
						"system": false,
						"type": "text"
					},
					{
						"hidden": false,
						"id": "number3632866850",
						"max": null,
						"min": null,
						"name": "rating",
						"onlyInt": false,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "number4280494897",
						"max": null,
						"min": 0,
						"name": "games",
						"onlyInt": true,
						"presentable": false,
						"required": false,
						"system": false,
						"type": "number"
					},
					{
						"hidden": false,
						"id": "autodate2990389176",
						"name": "created",
						"onCreate": true,
						"onUpdate": false,
						"presentable": false,
						"system": false,
						"type": "autodate"
					},
					{
						"hidden": false,
						"id": "autodate3332085495",
						"name": "updated",
						"onCreate": true,
						"onUpdate": true,
						"presentable": false,
						"system": false,
						"type": "autodate"
					}
				],
				"id": "pbc_1608874019",
				"indexes": [
					"CREATE UNIQUE INDEX ` + "`" + `idx_ratings_gamertag_gametype_unique` + "`" + ` ON ` + "`" + `ratings` + "`" + ` (gamertag, gametype)",
					"CREATE INDEX ` + "`" + `idx_ratings_gametype_rating` + "`" + ` ON ` + "`" + `ratings` + "`" + ` (gametype, rating)"
				],
				"listRule": "@request.auth.id != \"\"",
				"name": "ratings",
				"system": false,
				"type": "base",
				"updateRule": null,
				"viewRule": "@request.auth.id != \"\""
			}
		]`

		return app.ImportCollectionsByMarshaledJSON([]byte(jsonData), false)
	}, func(app core.App) error {
		return nil
	})
}
