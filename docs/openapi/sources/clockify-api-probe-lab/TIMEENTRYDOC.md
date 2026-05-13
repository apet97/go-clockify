Add a new time entry
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Request Body schema: application/json
required
billable	
boolean
Default: false
Indicates whether a time entry is billable or not.

customAttributes	
Array of objects (CreateCustomAttributeRequest) [ 0 .. 10 ] items
Default: "##default"
Represents a list of create custom field request objects.

Array ([ 0 .. 10 ] items)
name
required
string non-empty
Default: "##default"
Represents custom attribute name.

namespace
required
string non-empty
Default: "##default"
Represents custom attribute namespace.

value
required
string non-empty
Default: "##default"
Represents custom attribute value.

customFields	
Array of objects (UpdateCustomFieldRequest) [ 0 .. 50 ] items
Default: "##default"
Represents a list of value objects for user’s custom fields.

Array ([ 0 .. 50 ] items)
customFieldId
required
string
Default: "##default"
Represents custom field identifier across the system.

sourceType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "TIMEENTRY"
Represents a custom field value source type.

value	
object
Default: "##default"
Represents a custom field's value.

description	
string <= 3000 characters
Default: "##default"
Represents time entry description.

end	
string <date-time>
Represents an end date in yyyy-MM-ddThh:mm:ssZ format.

projectId	
string
Default: "##default"
Represents a project identifier across the system.

start	
string <date-time>
Represents a start date in yyyy-MM-ddThh:mm:ssZ format.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag ids.

taskId	
string
Default: "##default"
Represents a task identifier across the system.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK"
Valid time entry type.

Responses
201 Created
Response Schema: application/json
billable	
boolean
Default: false
Indicates whether a time entry is billable.

customFieldValues	
Array of objects (CustomFieldValueDtoV1)
Default: "##default"
Represents a list of custom field value objects.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

timeEntryId	
string
Default: "##default"
Represents time entry identifier across the system.

type	
string
Default: "##default"
Represents a custom field value source type.

value	
object
Default: "##default"
Represents custom field value.

description	
string
Default: "##default"
Represents time entry description.

id	
string
Default: "##default"
Represents time entry identifier across the system.

isLocked	
boolean
Default: false
Represents whether time entry is locked for modification.

kioskId	
string
Default: "##default"
Represents kiosk identifier across the system.

projectId	
string
Default: "##default"
Represents project identifier across the system.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag identifiers across the system.

taskId	
string
Default: "##default"
Represents task identifier across the system.

timeInterval	
object (TimeIntervalDtoV1)
Default: "##default"
Represents a time interval object.

duration	
string
Default: "##default"
Represents a time duration.

end	
string <date-time>
Represents an end date in yyyy-MM-ddThh:mm:ssZ format.

start	
string <date-time>
Represents a start date in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK" "HOLIDAY" "TIME_OFF"
Represents a time entry type enum.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/time-entries
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-entries
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"billable": false,
"customAttributes": "##default",
"customFields": "##default",
"description": "This is a sample time entry description.",
"end": "2021-01-01T00:00:00Z",
"projectId": "25b687e29ae1f428e7ebe123",
"start": "2020-01-01T00:00:00Z",
"tagIds": [
"321r77ddd3fcab07cfbb567y",
"44x777ddd3fcab07cfbb88f"
],
"taskId": "54m377ddd3fcab07cfbb432w",
"type": "REGULAR"
}
Response samples
201
Content type
application/json

Copy
Expand allCollapse all
{
"billable": false,
"customFieldValues": "##default",
"description": "This is a sample time entry description.",
"id": "64c777ddd3fcab07cfbb210c",
"isLocked": false,
"kioskId": "94c777ddd3fcab07cfbb210d",
"projectId": "25b687e29ae1f428e7ebe123",
"tagIds": [
"321r77ddd3fcab07cfbb567y",
"44x777ddd3fcab07cfbb88f"
],
"taskId": "54m377ddd3fcab07cfbb432w",
"timeInterval": "##default",
"type": "BREAK",
"userId": "5a0ab5acb07987125438b60f",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Mark time entries as invoiced
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Request Body schema: application/json
required
invoiced
required
boolean
Default: false
Indicates whether time entry is invoiced or not.

timeEntryIds
required
Array of objects (TimeEntryId) non-empty unique
Default: "##default"
Represents a list of invoiced time entry ids

Array (non-empty)
dateOfCreationFromObjectId	
string <date-time>
Responses
200 OK

patch
/v1/workspaces/{workspaceId}/time-entries/invoiced
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-entries/invoiced
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"invoiced": false,
"timeEntryIds": [
"54m377ddd3fcab07cfbb432w",
"25b687e29ae1f428e7ebe123"
]
}
Get all in progress time entries on a workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
query Parameters
page	
integer <int32> >= 1
Default: 1
page-size	
integer <int32> [ 1 .. 1000 ]
Default: 10
Responses
200 OK
Response Schema: */*
Array 
billable	
boolean
Default: false
Indicates whether a time entry is billable.

costRate	
object (RateDtoV1)
Default: "##default"
Represents hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

customFieldValues	
Array of objects (CustomFieldValueDtoV1)
Default: "##default"
Represents a list of custom field value objects.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

timeEntryId	
string
Default: "##default"
Represents time entry identifier across the system.

type	
string
Default: "##default"
Represents a custom field value source type.

value	
object
Default: "##default"
Represents custom field value.

description	
string
Default: "##default"
Represents time entry description.

hourlyRate	
object (RateDtoV1)
Default: "##default"
Represents hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

id	
string
Default: "##default"
Represents time entry identifier across the system.

isLocked	
boolean
Default: false
Represents whether time entry is locked for modification.

kioskId	
string
Default: "##default"
Represents kiosk identifier across the system.

projectId	
string
Default: "##default"
Represents project identifier across the system.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag identifiers across the system.

taskId	
string
Default: "##default"
Represents task identifier across the system.

timeInterval	
object (TimeIntervalDtoV1)
Default: "##default"
Represents a time interval object.

duration	
string
Default: "##default"
Represents a time duration.

end	
string <date-time>
Represents an end date in yyyy-MM-ddThh:mm:ssZ format.

start	
string <date-time>
Represents a start date in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK" "HOLIDAY" "TIME_OFF"
Represents a time entry type enum.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/time-entries/status/in-progress
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-entries/status/in-progress
Delete a time entry from a workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

id
required
string
Default: "##default"
Example: 64c777ddd3fcab07cfbb210c
Represents a time entry identifier across the system.

Responses
204 No Content

delete
/v1/workspaces/{workspaceId}/time-entries/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-entries/{id}
Get a specific time entry on a workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

id
required
string
Default: "##default"
Example: 64c777ddd3fcab07cfbb210c
Represents a time entry identifier across the system.

query Parameters
hydrated	
boolean
Default: false
Flag to set whether to include additional information of a time entry or not.

Responses
200 OK
Response Schema: application/json
billable	
boolean
Default: false
Indicates whether a time entry is billable.

costRate	
object (RateDtoV1)
Default: "##default"
Represents hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

customFieldValues	
Array of objects (CustomFieldValueDtoV1)
Default: "##default"
Represents a list of custom field value objects.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

timeEntryId	
string
Default: "##default"
Represents time entry identifier across the system.

type	
string
Default: "##default"
Represents a custom field value source type.

value	
object
Default: "##default"
Represents custom field value.

description	
string
Default: "##default"
Represents time entry description.

hourlyRate	
object (RateDtoV1)
Default: "##default"
Represents hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

id	
string
Default: "##default"
Represents time entry identifier across the system.

isLocked	
boolean
Default: false
Represents whether time entry is locked for modification.

kioskId	
string
Default: "##default"
Represents kiosk identifier across the system.

projectId	
string
Default: "##default"
Represents project identifier across the system.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag identifiers across the system.

taskId	
string
Default: "##default"
Represents task identifier across the system.

timeInterval	
object (TimeIntervalDtoV1)
Default: "##default"
Represents a time interval object.

duration	
string
Default: "##default"
Represents a time duration.

end	
string <date-time>
Represents an end date in yyyy-MM-ddThh:mm:ssZ format.

start	
string <date-time>
Represents a start date in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK" "HOLIDAY" "TIME_OFF"
Represents a time entry type enum.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/time-entries/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-entries/{id}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"billable": false,
"costRate": "##default",
"customFieldValues": "##default",
"description": "This is a sample time entry description.",
"hourlyRate": "##default",
"id": "64c777ddd3fcab07cfbb210c",
"isLocked": false,
"kioskId": "94c777ddd3fcab07cfbb210d",
"projectId": "25b687e29ae1f428e7ebe123",
"tagIds": [
"321r77ddd3fcab07cfbb567y",
"44x777ddd3fcab07cfbb88f"
],
"taskId": "54m377ddd3fcab07cfbb432w",
"timeInterval": "##default",
"type": "BREAK",
"userId": "5a0ab5acb07987125438b60f",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Update time entry on a workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

id
required
string
Default: "##default"
Example: 64c777ddd3fcab07cfbb210c
Represents a time entry identifier across the system.

Request Body schema: application/json
required
billable	
boolean
Default: false
Indicates whether a time entry is billable or not.

customFields	
Array of objects (UpdateCustomFieldRequest) [ 0 .. 50 ] items
Default: "##default"
Represents a list of value objects for user’s custom fields.

Array ([ 0 .. 50 ] items)
customFieldId
required
string
Default: "##default"
Represents custom field identifier across the system.

sourceType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "TIMEENTRY"
Represents a custom field value source type.

value	
object
Default: "##default"
Represents a custom field's value.

description	
string [ 0 .. 3000 ] characters
Default: "##default"
Represents time entry description.

end	
string <date-time>
Represents an end date in yyyy-MM-ddThh:mm:ssZ format.

projectId	
string
Default: "##default"
Represents a project identifier across the system.

start
required
string <date-time>
Represents a start date in yyyy-MM-ddThh:mm:ssZ format.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag ids.

taskId	
string
Default: "##default"
Represents a task identifier across the system.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK"
Responses
200 OK
Response Schema: application/json
billable	
boolean
Default: false
Indicates whether a time entry is billable.

customFieldValues	
Array of objects (CustomFieldValueDtoV1)
Default: "##default"
Represents a list of custom field value objects.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

timeEntryId	
string
Default: "##default"
Represents time entry identifier across the system.

type	
string
Default: "##default"
Represents a custom field value source type.

value	
object
Default: "##default"
Represents custom field value.

description	
string
Default: "##default"
Represents time entry description.

id	
string
Default: "##default"
Represents time entry identifier across the system.

isLocked	
boolean
Default: false
Represents whether time entry is locked for modification.

kioskId	
string
Default: "##default"
Represents kiosk identifier across the system.

projectId	
string
Default: "##default"
Represents project identifier across the system.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag identifiers across the system.

taskId	
string
Default: "##default"
Represents task identifier across the system.

timeInterval	
object (TimeIntervalDtoV1)
Default: "##default"
Represents a time interval object.

duration	
string
Default: "##default"
Represents a time duration.

end	
string <date-time>
Represents an end date in yyyy-MM-ddThh:mm:ssZ format.

start	
string <date-time>
Represents a start date in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK" "HOLIDAY" "TIME_OFF"
Represents a time entry type enum.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/time-entries/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-entries/{id}
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
"billable": false,
"customFieldValues": "##default",
"description": "This is a sample time entry description.",
"id": "64c777ddd3fcab07cfbb210c",
"isLocked": false,
"kioskId": "94c777ddd3fcab07cfbb210d",
"projectId": "25b687e29ae1f428e7ebe123",
"tagIds": [
"321r77ddd3fcab07cfbb567y",
"44x777ddd3fcab07cfbb88f"
],
"taskId": "54m377ddd3fcab07cfbb432w",
"timeInterval": "##default",
"type": "BREAK",
"userId": "5a0ab5acb07987125438b60f",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Delete all time entries for a user on a workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

userId
required
string
Default: "##default"
Example: 5a0ab5acb07987125438b60f
Represents a user identifier across the system.

query Parameters
time-entry-ids
required
Array of strings
Default: "##default"
Example: time-entry-ids=5a0ab5acb07987125438b60f
Represents a list of time entry ids to delete.

Responses
200 OK
Response Schema: application/json
Array 
billable	
boolean
Default: false
Indicates whether a time entry is billable.

customFieldValues	
Array of objects (CustomFieldValueDtoV1)
Default: "##default"
Represents a list of custom field value objects.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

timeEntryId	
string
Default: "##default"
Represents time entry identifier across the system.

type	
string
Default: "##default"
Represents a custom field value source type.

value	
object
Default: "##default"
Represents custom field value.

description	
string
Default: "##default"
Represents time entry description.

id	
string
Default: "##default"
Represents time entry identifier across the system.

isLocked	
boolean
Default: false
Represents whether time entry is locked for modification.

kioskId	
string
Default: "##default"
Represents kiosk identifier across the system.

projectId	
string
Default: "##default"
Represents project identifier across the system.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag identifiers across the system.

taskId	
string
Default: "##default"
Represents task identifier across the system.

timeInterval	
object (TimeIntervalDtoV1)
Default: "##default"
Represents a time interval object.

duration	
string
Default: "##default"
Represents a time duration.

end	
string <date-time>
Represents an end date in yyyy-MM-ddThh:mm:ssZ format.

start	
string <date-time>
Represents a start date in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK" "HOLIDAY" "TIME_OFF"
Represents a time entry type enum.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


delete
/v1/workspaces/{workspaceId}/user/{userId}/time-entries
https://api.clockify.me/api/v1/workspaces/{workspaceId}/user/{userId}/time-entries
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"billable": false,
"customFieldValues": "##default",
"description": "This is a sample time entry description.",
"id": "64c777ddd3fcab07cfbb210c",
"isLocked": false,
"kioskId": "94c777ddd3fcab07cfbb210d",
"projectId": "25b687e29ae1f428e7ebe123",
"tagIds": [
"321r77ddd3fcab07cfbb567y",
"44x777ddd3fcab07cfbb88f"
],
"taskId": "54m377ddd3fcab07cfbb432w",
"timeInterval": "##default",
"type": "BREAK",
"userId": "5a0ab5acb07987125438b60f",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Get time entries for a user on a workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

userId
required
string
Default: "##default"
Example: 5a0ab5acb07987125438b60f
Represents a user identifier across the system.

query Parameters
description	
string
Default: "##default"
Example: description=Description keywords
Represents a term for searching time entries by description.

start	
string
Default: "##default"
Example: start=2020-01-01T00:00:00Z
Represents a start date in the yyyy-MM-ddThh:mm:ssZ format.

end	
string
Default: "##default"
Example: end=2021-01-01T00:00:00Z
Represents an end date in the yyyy-MM-ddThh:mm:ssZ format.

project	
string
Default: "##default"
Example: project=5b641568b07987035750505e
If provided, you'll get a filtered list of time entries that matches the provided string in their project id.

task	
string
Default: "##default"
Example: task=64c777ddd3fcab07cfbb210c
If provided, you'll get a filtered list of time entries that matches the provided string in their task id.

tags	
Array of strings unique
Default: "##default"
Example: tags=5e4117fe8c625f38930d57b7&tags=7e4117fe8c625f38930d57b8
If provided, you'll get a filtered list of time entries that matches the provided string(s) in their tag id(s).

project-required	
boolean
Default: false
Flag to set whether to only get time entries which have a project.

task-required	
boolean
Default: false
Flag to set whether to only get time entries which have tasks.

hydrated	
boolean
Default: false
Flag to set whether to include additional information on time entries or not.

page	
integer <int32>
Default: 1
Example: page=1
Page number.

page-size	
integer <int32> >= 1
Default: 50
Example: page-size=50
Page size.

in-progress	
string
Default: "##default"
Flag to set whether to filter only in progress time entries.

get-week-before	
string
Default: "##default"
Example: get-week-before=2020-01-01T00:00:00Z
Valid yyyy-MM-ddThh:mm:ssZ format date. If provided, filters results within the week before the datetime provided and only those entries with assigned project or task.

Responses
200 OK

get
/v1/workspaces/{workspaceId}/user/{userId}/time-entries
https://api.clockify.me/api/v1/workspaces/{workspaceId}/user/{userId}/time-entries
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"billable": false,
"costRate": "##default",
"customFieldValues": "##default",
"description": "This is a sample time entry description.",
"hourlyRate": "##default",
"id": "64c777ddd3fcab07cfbb210c",
"isLocked": false,
"kioskId": "94c777ddd3fcab07cfbb210d",
"projectId": "25b687e29ae1f428e7ebe123",
"tagIds": [
"321r77ddd3fcab07cfbb567y",
"44x777ddd3fcab07cfbb88f"
],
"taskId": "54m377ddd3fcab07cfbb432w",
"timeInterval": "##default",
"type": "BREAK",
"userId": "5a0ab5acb07987125438b60f",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Stop a currently running timer on a workspace for a user
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

userId
required
string
Default: "##default"
Example: 5a0ab5acb07987125438b60f
Represents a user identifier across the system.

Request Body schema: application/json
required
end
required
string <date-time>
Represents an end date in the yyyy-MM-ddThh:mm:ssZ format.

Responses
200 OK
Response Schema: application/json
billable	
boolean
Default: false
Indicates whether a time entry is billable.

customFieldValues	
Array of objects (CustomFieldValueDtoV1)
Default: "##default"
Represents a list of custom field value objects.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

timeEntryId	
string
Default: "##default"
Represents time entry identifier across the system.

type	
string
Default: "##default"
Represents a custom field value source type.

value	
object
Default: "##default"
Represents custom field value.

description	
string
Default: "##default"
Represents time entry description.

id	
string
Default: "##default"
Represents time entry identifier across the system.

isLocked	
boolean
Default: false
Represents whether time entry is locked for modification.

kioskId	
string
Default: "##default"
Represents kiosk identifier across the system.

projectId	
string
Default: "##default"
Represents project identifier across the system.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag identifiers across the system.

taskId	
string
Default: "##default"
Represents task identifier across the system.

timeInterval	
object (TimeIntervalDtoV1)
Default: "##default"
Represents a time interval object.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK" "HOLIDAY" "TIME_OFF"
Represents a time entry type enum.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


patch
/v1/workspaces/{workspaceId}/user/{userId}/time-entries
https://api.clockify.me/api/v1/workspaces/{workspaceId}/user/{userId}/time-entries
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
"billable": false,
"customFieldValues": "##default",
"description": "This is a sample time entry description.",
"id": "64c777ddd3fcab07cfbb210c",
"isLocked": false,
"kioskId": "94c777ddd3fcab07cfbb210d",
"projectId": "25b687e29ae1f428e7ebe123",
"tagIds": [
"321r77ddd3fcab07cfbb567y",
"44x777ddd3fcab07cfbb88f"
],
"taskId": "54m377ddd3fcab07cfbb432w",
"timeInterval": "##default",
"type": "BREAK",
"userId": "5a0ab5acb07987125438b60f",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Add a new time entry for another user on workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

userId
required
string
Default: "##default"
Example: 5a0ab5acb07987125438b60f
Represents a user identifier across the system.

query Parameters
from-entry	
string
Default: "##default"
Example: from-entry=64c777ddd3fcab07cfbb210c
Represents a time entry identifier across the system.

Request Body schema: application/json
required
billable	
boolean
Default: false
Indicates whether a time entry is billable or not.

customAttributes	
Array of objects (CreateCustomAttributeRequest) [ 0 .. 10 ] items
Default: "##default"
Represents a list of create custom field request objects.

Array ([ 0 .. 10 ] items)
name
required
string non-empty
Default: "##default"
Represents custom attribute name.

namespace
required
string non-empty
Default: "##default"
Represents custom attribute namespace.

value
required
string non-empty
Default: "##default"
Represents custom attribute value.

customFields	
Array of objects (UpdateCustomFieldRequest) [ 0 .. 50 ] items
Default: "##default"
Represents a list of value objects for user’s custom fields.

Array ([ 0 .. 50 ] items)
customFieldId
required
string
Default: "##default"
Represents custom field identifier across the system.

sourceType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "TIMEENTRY"
Represents a custom field value source type.

value	
object
Default: "##default"
Represents a custom field's value.

description	
string <= 3000 characters
Default: "##default"
Represents time entry description.

end	
string <date-time>
Represents an end date in yyyy-MM-ddThh:mm:ssZ format.

projectId	
string
Default: "##default"
Represents a project identifier across the system.

start	
string <date-time>
Represents a start date in yyyy-MM-ddThh:mm:ssZ format.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag ids.

taskId	
string
Default: "##default"
Represents a task identifier across the system.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK"
Valid time entry type.

Responses
201 Created
Response Schema: application/json
billable	
boolean
Default: false
Indicates whether a time entry is billable.

customFieldValues	
Array of objects (CustomFieldValueDtoV1)
Default: "##default"
Represents a list of custom field value objects.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

timeEntryId	
string
Default: "##default"
Represents time entry identifier across the system.

type	
string
Default: "##default"
Represents a custom field value source type.

value	
object
Default: "##default"
Represents custom field value.

description	
string
Default: "##default"
Represents time entry description.

id	
string
Default: "##default"
Represents time entry identifier across the system.

isLocked	
boolean
Default: false
Represents whether time entry is locked for modification.

kioskId	
string
Default: "##default"
Represents kiosk identifier across the system.

projectId	
string
Default: "##default"
Represents project identifier across the system.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag identifiers across the system.

taskId	
string
Default: "##default"
Represents task identifier across the system.

timeInterval	
object (TimeIntervalDtoV1)
Default: "##default"
Represents a time interval object.

duration	
string
Default: "##default"
Represents a time duration.

end	
string <date-time>
Represents an end date in yyyy-MM-ddThh:mm:ssZ format.

start	
string <date-time>
Represents a start date in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK" "HOLIDAY" "TIME_OFF"
Represents a time entry type enum.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/user/{userId}/time-entries
https://api.clockify.me/api/v1/workspaces/{workspaceId}/user/{userId}/time-entries
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"billable": false,
"customAttributes": "##default",
"customFields": "##default",
"description": "This is a sample time entry description.",
"end": "2021-01-01T00:00:00Z",
"projectId": "25b687e29ae1f428e7ebe123",
"start": "2020-01-01T00:00:00Z",
"tagIds": [
"321r77ddd3fcab07cfbb567y",
"44x777ddd3fcab07cfbb88f"
],
"taskId": "54m377ddd3fcab07cfbb432w",
"type": "REGULAR"
}
Response samples
201
Content type
application/json

Copy
Expand allCollapse all
{
"billable": false,
"customFieldValues": "##default",
"description": "This is a sample time entry description.",
"id": "64c777ddd3fcab07cfbb210c",
"isLocked": false,
"kioskId": "94c777ddd3fcab07cfbb210d",
"projectId": "25b687e29ae1f428e7ebe123",
"tagIds": [
"321r77ddd3fcab07cfbb567y",
"44x777ddd3fcab07cfbb88f"
],
"taskId": "54m377ddd3fcab07cfbb432w",
"timeInterval": "##default",
"type": "BREAK",
"userId": "5a0ab5acb07987125438b60f",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Bulk edit time entries
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

userId
required
string
Default: "##default"
Example: 5a0ab5acb07987125438b60f
Represents a user identifier across the system.

query Parameters
hydrated	
boolean
Default: false
If set to true, results will contain additional information about the time entry.

Request Body schema: application/json
required
Array (non-empty)
billable	
boolean
Default: false
Indicates whether a time entry is billable or not.

customFields	
Array of objects (UpdateCustomFieldRequest) [ 0 .. 50 ] items
Default: "##default"
description	
string [ 0 .. 3000 ] characters
Default: "##default"
Represents a time entry description.

end	
string <date-time>
Represents an end date in the yyyy-MM-ddThh:mm:ssZ format.

id
required
string non-empty
Default: "##default"
Represents a time entry identifier across the system.

projectId	
string
Default: "##default"
Represents a project identifier across the system.

start	
string <date-time>
Represents a start date in the yyyy-MM-ddThh:mm:ssZ format.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag ids.

taskId	
string
Default: "##default"
Represents a task identifier across the system.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK"
Responses
200 OK
Response Schema: application/json
Array 
billable	
boolean
Default: false
Indicates whether a time entry is billable.

customFieldValues	
Array of objects (CustomFieldValueDtoV1)
Default: "##default"
Represents a list of custom field value objects.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

timeEntryId	
string
Default: "##default"
Represents time entry identifier across the system.

type	
string
Default: "##default"
Represents a custom field value source type.

value	
object
Default: "##default"
Represents custom field value.

description	
string
Default: "##default"
Represents time entry description.

id	
string
Default: "##default"
Represents time entry identifier across the system.

isLocked	
boolean
Default: false
Represents whether time entry is locked for modification.

kioskId	
string
Default: "##default"
Represents kiosk identifier across the system.

projectId	
string
Default: "##default"
Represents project identifier across the system.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag identifiers across the system.

taskId	
string
Default: "##default"
Represents task identifier across the system.

timeInterval	
object (TimeIntervalDtoV1)
Default: "##default"
Represents a time interval object.

duration	
string
Default: "##default"
Represents a time duration.

end	
string <date-time>
Represents an end date in yyyy-MM-ddThh:mm:ssZ format.

start	
string <date-time>
Represents a start date in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK" "HOLIDAY" "TIME_OFF"
Represents a time entry type enum.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/user/{userId}/time-entries
https://api.clockify.me/api/v1/workspaces/{workspaceId}/user/{userId}/time-entries
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
[
{
"billable": false,
"customFieldValues": "##default",
"description": "This is a sample time entry description.",
"id": "64c777ddd3fcab07cfbb210c",
"isLocked": false,
"kioskId": "94c777ddd3fcab07cfbb210d",
"projectId": "25b687e29ae1f428e7ebe123",
"tagIds": [
"321r77ddd3fcab07cfbb567y",
"44x777ddd3fcab07cfbb88f"
],
"taskId": "54m377ddd3fcab07cfbb432w",
"timeInterval": "##default",
"type": "BREAK",
"userId": "5a0ab5acb07987125438b60f",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Duplicate a time entry
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

userId
required
string
Default: "##default"
Example: 5a0ab5acb07987125438b60f
Represents a user identifier across the system.

id
required
string
Default: "##default"
Example: 8j39fn9307hh5125439g2ast
Represents a time entry identifier across the system.

Responses
201 Created
Response Schema: application/json
billable	
boolean
Default: false
Indicates whether a time entry is billable.

customFieldValues	
Array of objects (CustomFieldValueDtoV1)
Default: "##default"
Represents a list of custom field value objects.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents custom field name.

timeEntryId	
string
Default: "##default"
Represents time entry identifier across the system.

type	
string
Default: "##default"
Represents a custom field value source type.

value	
object
Default: "##default"
Represents custom field value.

description	
string
Default: "##default"
Represents time entry description.

id	
string
Default: "##default"
Represents time entry identifier across the system.

isLocked	
boolean
Default: false
Represents whether time entry is locked for modification.

kioskId	
string
Default: "##default"
Represents kiosk identifier across the system.

projectId	
string
Default: "##default"
Represents project identifier across the system.

tagIds	
Array of strings
Default: "##default"
Represents a list of tag identifiers across the system.

taskId	
string
Default: "##default"
Represents task identifier across the system.

timeInterval	
object (TimeIntervalDtoV1)
Default: "##default"
Represents a time interval object.

duration	
string
Default: "##default"
Represents a time duration.

end	
string <date-time>
Represents an end date in yyyy-MM-ddThh:mm:ssZ format.

start	
string <date-time>
Represents a start date in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "REGULAR" "BREAK" "HOLIDAY" "TIME_OFF"
Represents a time entry type enum.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/user/{userId}/time-entries/{id}/duplicate
https://api.clockify.me/api/v1/workspaces/{workspaceId}/user/{userId}/time-entries/{id}/duplicate
Response samples
201
Content type
application/json

Copy
Expand allCollapse all
{
"billable": false,
"customFieldValues": "##default",
"description": "This is a sample time entry description.",
"id": "64c777ddd3fcab07cfbb210c",
"isLocked": false,
"kioskId": "94c777ddd3fcab07cfbb210d",
"projectId": "25b687e29ae1f428e7ebe123",
"tagIds": [
"321r77ddd3fcab07cfbb567y",
"44x777ddd3fcab07cfbb88f"
],
"taskId": "54m377ddd3fcab07cfbb432w",
"timeInterval": "##default",
"type": "BREAK",
"userId": "5a0ab5acb07987125438b60f",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
