Get custom fields on a workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

query Parameters
name	
string
Default: "##default"
Example: name=location
If provided, you'll get a filtered list of custom fields that contain the provided string in their name.

status	
string
Enum: "INACTIVE" "VISIBLE" "INVISIBLE"
Example: status=VISIBLE
If provided, you'll get a filtered list of custom fields that matches the provided string with the custom field status.

entity-type	
string
Example: entity-type=TIMEENTRY&entity-type=USER
If provided, you'll get a filtered list of custom fields that matches the provided string with the custom field entity type.

Responses
200 OK
Response Schema: application/json
Array 
allowedValues	
Array of strings
Default: "##default"
Represents a list of custom field's allowed values.

description	
string
Default: "##default"
Represents custom field description.

entityType	
string
Default: "##default"
Represents custom field entity type

id	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

onlyAdminCanEdit	
boolean
Default: false
Flag to set whether custom field is modifiable only by admin users.

placeholder	
string
Default: "##default"
Represents custom field placeholder value.

projectDefaultValues	
Array of objects (CustomFieldDefaultValuesDtoV1)
Default: "##default"
Represents a list of custom field default values data transfer objects.

Array 
projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
string
Default: "##default"
Represents custom field status

value	
object
Default: "##default"
Represents a custom field's default value

required	
boolean
Default: false
Flag to set whether custom field is mandatory or not.

status	
string
Default: "##default"
Represents custom field status

type	
string
Default: "##default"
Represents custom field type.

workspaceDefaultValue	
object
Default: "##default"
Represents a custom field's default value in the workspace.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/custom-fields
https://api.clockify.me/api/v1/workspaces/{workspaceId}/custom-fields
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"allowedValues": [
"New York",
"London",
"Manila",
"Sydney",
"Belgrade"
],
"description": "This field contains a location.",
"entityType": "USER",
"id": "44a687e29ae1f428e7ebe305",
"name": "location",
"onlyAdminCanEdit": false,
"placeholder": "Location",
"projectDefaultValues": "##default",
"required": false,
"status": "VISIBLE",
"type": "DROPDOWN_MULTIPLE",
"workspaceDefaultValue": "Manila",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Create custom fields on a workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents workspace identifier across the system.

Request Body schema: application/json
required
allowedValues	
Array of strings
Default: "##default"
Represents a list of custom field's allowed values.

description	
string
Default: "##default"
Represents custom field description.

entityType	
string
Default: "##default"
Enum: "TIMEENTRY" "USER"
Represents custom field entity type

name
required
string
Default: "##default"
Represents custom field name.

onlyAdminCanEdit	
boolean
Default: false
Flag to set whether custom field is modifiable only by admin users.

placeholder	
string
Default: "##default"
Represents custom field placeholder value.

status	
string
Default: "##default"
Enum: "INACTIVE" "VISIBLE" "INVISIBLE"
Represents custom field status

type
required
string
Default: "##default"
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
Represents custom field type.

workspaceDefaultValue	
object
Default: "##default"
Represents a custom field's default value in the workspace.

if type = NUMBER, then value must be a number
if type = DROPDOWN_MULTIPLE, value must be a list
if type = CHECKBOX, value must be true/false
otherwise any string
Responses
201 Created
Response Schema: */*
allowedValues	
Array of strings
Default: "##default"
Represents a list of custom field's allowed values.

description	
string
Default: "##default"
Represents custom field description.

entityType	
string
Default: "##default"
Represents custom field entity type

id	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

onlyAdminCanEdit	
boolean
Default: false
Flag to set whether custom field is modifiable only by admin users.

placeholder	
string
Default: "##default"
Represents custom field placeholder value.

projectDefaultValues	
Array of objects (CustomFieldDefaultValuesDtoV1)
Default: "##default"
Represents a list of custom field default values data transfer objects.

Array 
projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
string
Default: "##default"
Represents custom field status

value	
object
Default: "##default"
Represents a custom field's default value

required	
boolean
Default: false
Flag to set whether custom field is mandatory or not.

status	
string
Default: "##default"
Represents custom field status

type	
string
Default: "##default"
Represents custom field type.

workspaceDefaultValue	
object
Default: "##default"
Represents a custom field's default value in the workspace.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/custom-fields
https://api.clockify.me/api/v1/workspaces/{workspaceId}/custom-fields
Request samples
Payload
Content type
application/json

Copy
"##default"
Delete a custom field
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents workspace identifier across the system.

customFieldId
required
string
Default: "##default"
Example: 26a687e29ae1f428e7ebe101
Represents custom field identifier across the system.

Responses
204 No Content

delete
/v1/workspaces/{workspaceId}/custom-fields/{customFieldId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/custom-fields/{customFieldId}
Update custom field on workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

customFieldId
required
string
Default: "##default"
Example: 26a687e29ae1f428e7ebe101
Represents a custom field identifier across the system.

Request Body schema: application/json
required
allowedValues	
Array of strings
Default: "##default"
Represents a list of custom field's allowed values.

description	
string
Default: "##default"
Represents a custom field description.

name
required
string [ 2 .. 250 ] characters
Default: "##default"
Represents a custom field name.

onlyAdminCanEdit	
boolean
Default: false
Flag to set whether custom field is modifiable only by admin users.

placeholder	
string
Default: "##default"
Represents a custom field placeholder value.

required	
boolean
Default: false
Flag to set whether custom field is mandatory or not.

status	
string
Default: "##default"
Enum: "INACTIVE" "VISIBLE" "INVISIBLE"
Represents a custom field status

type
required
string
Default: "##default"
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
Represents a custom field type.

workspaceDefaultValue	
object
Default: "##default"
Represents a custom field's default value in the workspace.

Responses
200 OK
Response Schema: application/json
allowedValues	
Array of strings
Default: "##default"
Represents a list of custom field's allowed values.

description	
string
Default: "##default"
Represents custom field description.

entityType	
string
Default: "##default"
Represents custom field entity type

id	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

onlyAdminCanEdit	
boolean
Default: false
Flag to set whether custom field is modifiable only by admin users.

placeholder	
string
Default: "##default"
Represents custom field placeholder value.

projectDefaultValues	
Array of objects (CustomFieldDefaultValuesDtoV1)
Default: "##default"
Represents a list of custom field default values data transfer objects.

Array 
projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
string
Default: "##default"
Represents custom field status

value	
object
Default: "##default"
Represents a custom field's default value

required	
boolean
Default: false
Flag to set whether custom field is mandatory or not.

status	
string
Default: "##default"
Represents custom field status

type	
string
Default: "##default"
Represents custom field type.

workspaceDefaultValue	
object
Default: "##default"
Represents a custom field's default value in the workspace.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/custom-fields/{customFieldId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/custom-fields/{customFieldId}
Request samples
Payload
Content type
application/json

Copy
"##default"
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"allowedValues": [
"New York",
"London",
"Manila",
"Sydney",
"Belgrade"
],
"description": "This field contains a location.",
"entityType": "USER",
"id": "44a687e29ae1f428e7ebe305",
"name": "location",
"onlyAdminCanEdit": false,
"placeholder": "Location",
"projectDefaultValues": "##default",
"required": false,
"status": "VISIBLE",
"type": "DROPDOWN_MULTIPLE",
"workspaceDefaultValue": "Manila",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Get custom fields on a project
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

projectId
required
string
Default: "##default"
Example: 5b641568b07987035750505e
Represents a project identifier across the system.

query Parameters
status	
string
Enum: "INACTIVE" "VISIBLE" "INVISIBLE"
Example: status=INACTIVE
If provided, you'll get a filtered list of custom fields that matches the provided string with the custom field status.

entity-type	
string
Example: entity-type=TIMEENTRY
If provided, you'll get a filtered list of custom fields that matches the provided string with the custom field entity type.

Responses
200 OK
Response Schema: application/json
Array 
allowedValues	
Array of strings
Default: "##default"
Represents a list of custom field's allowed values.

description	
string
Default: "##default"
Represents custom field description.

entityType	
string
Default: "##default"
Represents custom field entity type

id	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

onlyAdminCanEdit	
boolean
Default: false
Flag to set whether custom field is modifiable only by admin users.

placeholder	
string
Default: "##default"
Represents custom field placeholder value.

projectDefaultValues	
Array of objects (CustomFieldDefaultValuesDtoV1)
Default: "##default"
Represents a list of custom field default values data transfer objects.

Array 
projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
string
Default: "##default"
Represents custom field status

value	
object
Default: "##default"
Represents a custom field's default value

required	
boolean
Default: false
Flag to set whether custom field is mandatory or not.

status	
string
Default: "##default"
Represents custom field status

type	
string
Default: "##default"
Represents custom field type.

workspaceDefaultValue	
object
Default: "##default"
Represents a custom field's default value in the workspace.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/projects/{projectId}/custom-fields
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/custom-fields
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"allowedValues": [
"New York",
"London",
"Manila",
"Sydney",
"Belgrade"
],
"description": "This field contains a location.",
"entityType": "USER",
"id": "44a687e29ae1f428e7ebe305",
"name": "location",
"onlyAdminCanEdit": false,
"placeholder": "Location",
"projectDefaultValues": "##default",
"required": false,
"status": "VISIBLE",
"type": "DROPDOWN_MULTIPLE",
"workspaceDefaultValue": "Manila",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Remove custom field from a project
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

projectId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a project identifier across the system.

customFieldId
required
string
Default: "##default"
Example: 26a687e29ae1f428e7ebe101
Represents a custom field identifier across the system.

Responses
200 OK
Response Schema: application/json
allowedValues	
Array of strings
Default: "##default"
Represents a list of custom field's allowed values.

description	
string
Default: "##default"
Represents custom field description.

entityType	
string
Default: "##default"
Represents custom field entity type

id	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

onlyAdminCanEdit	
boolean
Default: false
Flag to set whether custom field is modifiable only by admin users.

placeholder	
string
Default: "##default"
Represents custom field placeholder value.

projectDefaultValues	
Array of objects (CustomFieldDefaultValuesDtoV1)
Default: "##default"
Represents a list of custom field default values data transfer objects.

Array 
projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
string
Default: "##default"
Represents custom field status

value	
object
Default: "##default"
Represents a custom field's default value

required	
boolean
Default: false
Flag to set whether custom field is mandatory or not.

status	
string
Default: "##default"
Represents custom field status

type	
string
Default: "##default"
Represents custom field type.

workspaceDefaultValue	
object
Default: "##default"
Represents a custom field's default value in the workspace.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


delete
/v1/workspaces/{workspaceId}/projects/{projectId}/custom-fields/{customFieldId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/custom-fields/{customFieldId}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"allowedValues": [
"New York",
"London",
"Manila",
"Sydney",
"Belgrade"
],
"description": "This field contains a location.",
"entityType": "USER",
"id": "44a687e29ae1f428e7ebe305",
"name": "location",
"onlyAdminCanEdit": false,
"placeholder": "Location",
"projectDefaultValues": "##default",
"required": false,
"status": "VISIBLE",
"type": "DROPDOWN_MULTIPLE",
"workspaceDefaultValue": "Manila",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Update custom field on a project
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

projectId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a project identifier across the system.

customFieldId
required
string
Default: "##default"
Example: 26a687e29ae1f428e7ebe101
Represents a custom field identifier across the system.

Request Body schema: application/json
required
defaultValue	
object
Default: "##default"
Represents a custom field's default value.

status	
string
Default: "##default"
Enum: "INACTIVE" "VISIBLE" "INVISIBLE"
Represents a custom field status.

Responses
200 OK
Response Schema: application/json
allowedValues	
Array of strings
Default: "##default"
Represents a list of custom field's allowed values.

description	
string
Default: "##default"
Represents custom field description.

entityType	
string
Default: "##default"
Represents custom field entity type

id	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

onlyAdminCanEdit	
boolean
Default: false
Flag to set whether custom field is modifiable only by admin users.

placeholder	
string
Default: "##default"
Represents custom field placeholder value.

projectDefaultValues	
Array of objects (CustomFieldDefaultValuesDtoV1)
Default: "##default"
Represents a list of custom field default values data transfer objects.

Array 
projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
string
Default: "##default"
Represents custom field status

value	
object
Default: "##default"
Represents a custom field's default value

required	
boolean
Default: false
Flag to set whether custom field is mandatory or not.

status	
string
Default: "##default"
Represents custom field status

type	
string
Default: "##default"
Represents custom field type.

workspaceDefaultValue	
object
Default: "##default"
Represents a custom field's default value in the workspace.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


patch
/v1/workspaces/{workspaceId}/projects/{projectId}/custom-fields/{customFieldId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/custom-fields/{customFieldId}
Request samples
Payload
Content type
application/json

Copy
{
"defaultValue": "Manila",
"status": "VISIBLE"
}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"allowedValues": [
"New York",
"London",
"Manila",
"Sydney",
"Belgrade"
],
"description": "This field contains a location.",
"entityType": "USER",
"id": "44a687e29ae1f428e7ebe305",
"name": "location",
"onlyAdminCanEdit": false,
"placeholder": "Location",
"projectDefaultValues": "##default",
"required": false,
"status": "VISIBLE",
"type": "DROPDOWN_MULTIPLE",
"workspaceDefaultValue": "Manila",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
