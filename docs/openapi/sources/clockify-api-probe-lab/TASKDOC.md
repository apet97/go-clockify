Find tasks on a project
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
projectId
required
string
Default: "##default"
Example: 25b687e29ae1f428e7ebe123
Represents a project identifier across the system.

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
Example: name=Bugfixing
If provided, you'll get a filtered list of tasks that matches the provided string in their name.

strict-name-search	
boolean
Default: false
Flag to toggle on/off strict search mode. When set to true, search by name only will return tasks whose name exactly matches the string value given for the 'name' parameter. When set to false, results will also include tasks whose name contain the string value, but could be longer than the string value itself. For example, if there is a task with the name 'applications', and the search value is 'app', setting strict-name-search to true will not return that task in the results, whereas setting it to false will.

is-active	
boolean
Default: false
Filters search results whether task is active or not.

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

sort-column	
string
Enum: "ID" "NAME"
Example: sort-column=ID
Represents the column as criteria for sorting tasks.

sort-order	
string
Enum: "ASCENDING" "DESCENDING"
Example: sort-order=ASCENDING
Sorting mode.

Responses
200 OK
Response Schema: application/json
Array 
assigneeId	
string
Deprecated
assigneeIds	
Array of strings unique
Default: "##default"
Represents list of assignee ids for the task.

billable	
boolean
Default: false
Indicates whether a task is billable or not.

budgetEstimate	
integer <int64>
Represents a task budget estimate as long.

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

duration	
string
Default: "##default"
Represents a task duration.

estimate	
string
Default: "##default"
Represents a task duration estimate.

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
Represents task identifier across the system.

name	
string
Default: "##default"
Represents task name.

projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
object (TaskStatus)
Default: "##default"
Represents task status.

One of object
ACTIVE	
string
Enum: "ACTIVE" "DONE" "ALL"
ALL	
string
Enum: "ACTIVE" "DONE" "ALL"
DONE	
string
Enum: "ACTIVE" "DONE" "ALL"
active	
boolean
userGroupIds	
Array of strings unique
Default: "##default"
Represents list of user group ids for the task.


get
/v1/workspaces/{workspaceId}/projects/{projectId}/tasks
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/tasks
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"assigneeId": "string",
"assigneeIds": [
"45b687e29ae1f428e7ebe123",
"67s687e29ae1f428e7ebe678"
],
"billable": false,
"budgetEstimate": 10000,
"costRate": "##default",
"duration": "PT1H30M",
"estimate": "PT1H30M",
"hourlyRate": "##default",
"id": "57a687e29ae1f428e7ebe107",
"name": "Bugfixing",
"projectId": "25b687e29ae1f428e7ebe123",
"status": "DONE",
"userGroupIds": [
"67b687e29ae1f428e7ebe123",
"12s687e29ae1f428e7ebe678"
]
}
]
Add a new task on a project
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
projectId
required
string
Default: "##default"
Example: 25b687e29ae1f428e7ebe123
Represents a project identifier across the system.

workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

query Parameters
contains-assignee	
boolean
Default: true
Flag to set whether task will have assignee or none.

Request Body schema: application/json
required
assigneeId	
string
Deprecated
Default: "##default"
assigneeIds	
Array of strings unique
Default: "##default"
Represents list of assignee ids for the task.

budgetEstimate	
integer <int64> >= 0
Represents a task budget estimate as long.

estimate	
string
Default: "##default"
Represents a task duration estimate in ISO-8601 format.

id	
string
Default: "##default"
Represents task identifier across the system.

name
required
string [ 1 .. 1000 ] characters
Default: "##default"
Represents task name.

status	
string
Default: "##default"
Enum: "ACTIVE" "DONE" "ALL"
Represents task status.

userGroupIds	
Array of strings unique
Default: "##default"
Represents list of user group ids for the task.

Responses
201 Created
Response Schema: application/json
assigneeId	
string
Deprecated
assigneeIds	
Array of strings unique
Default: "##default"
Represents list of assignee ids for the task.

billable	
boolean
Default: false
Indicates whether a task is billable or not.

budgetEstimate	
integer <int64>
Represents a task budget estimate as long.

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

duration	
string
Default: "##default"
Represents a task duration.

estimate	
string
Default: "##default"
Represents a task duration estimate.

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
Represents task identifier across the system.

name	
string
Default: "##default"
Represents task name.

projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
object (TaskStatus)
Default: "##default"
Represents task status.

One of object
ACTIVE	
string
Enum: "ACTIVE" "DONE" "ALL"
ALL	
string
Enum: "ACTIVE" "DONE" "ALL"
DONE	
string
Enum: "ACTIVE" "DONE" "ALL"
active	
boolean
userGroupIds	
Array of strings unique
Default: "##default"
Represents list of user group ids for the task.


post
/v1/workspaces/{workspaceId}/projects/{projectId}/tasks
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/tasks
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"assigneeId": "##default",
"assigneeIds": [
"45b687e29ae1f428e7ebe123",
"67s687e29ae1f428e7ebe678"
],
"budgetEstimate": 10000,
"estimate": "PT1H30M",
"id": "57a687e29ae1f428e7ebe107",
"name": "Bugfixing",
"status": "DONE",
"userGroupIds": [
"67b687e29ae1f428e7ebe123",
"12s687e29ae1f428e7ebe678"
]
}
Response samples
201
Content type
application/json

Copy
Expand allCollapse all
{
"assigneeId": "string",
"assigneeIds": [
"45b687e29ae1f428e7ebe123",
"67s687e29ae1f428e7ebe678"
],
"billable": false,
"budgetEstimate": 10000,
"costRate": "##default",
"duration": "PT1H30M",
"estimate": "PT1H30M",
"hourlyRate": "##default",
"id": "57a687e29ae1f428e7ebe107",
"name": "Bugfixing",
"projectId": "25b687e29ae1f428e7ebe123",
"status": "DONE",
"userGroupIds": [
"67b687e29ae1f428e7ebe123",
"12s687e29ae1f428e7ebe678"
]
}
Update a task's cost rate
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
projectId
required
string
Default: "##default"
Example: 25b687e29ae1f428e7ebe123
Represents a project identifier across the system.

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
Example: 57a687e29ae1f428e7ebe107
Represents a task identifier across the system.

Request Body schema: application/json
required
amount
required
integer <int32> >= 0
Represents an amount as integer.

since	
string
Default: "##default"
Represents a date and time in yyyy-MM-ddThh:mm:ssZ format.

Responses
200 OK
Response Schema: application/json
assigneeId	
string
Deprecated
assigneeIds	
Array of strings unique
Default: "##default"
Represents list of assignee ids for the task.

billable	
boolean
Default: false
Indicates whether a task is billable or not.

budgetEstimate	
integer <int64>
Represents a task budget estimate as long.

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

duration	
string
Default: "##default"
Represents a task duration.

estimate	
string
Default: "##default"
Represents a task duration estimate.

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
Represents task identifier across the system.

name	
string
Default: "##default"
Represents task name.

projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
object (TaskStatus)
Default: "##default"
Represents task status.

One of object
ACTIVE	
string
Enum: "ACTIVE" "DONE" "ALL"
ALL	
string
Enum: "ACTIVE" "DONE" "ALL"
DONE	
string
Enum: "ACTIVE" "DONE" "ALL"
active	
boolean
userGroupIds	
Array of strings unique
Default: "##default"
Represents list of user group ids for the task.


put
/v1/workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/cost-rate
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/cost-rate
Request samples
Payload
Content type
application/json

Copy
{
"amount": 20000,
"since": "2020-01-01T00:00:00Z"
}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"assigneeId": "string",
"assigneeIds": [
"45b687e29ae1f428e7ebe123",
"67s687e29ae1f428e7ebe678"
],
"billable": false,
"budgetEstimate": 10000,
"costRate": "##default",
"duration": "PT1H30M",
"estimate": "PT1H30M",
"hourlyRate": "##default",
"id": "57a687e29ae1f428e7ebe107",
"name": "Bugfixing",
"projectId": "25b687e29ae1f428e7ebe123",
"status": "DONE",
"userGroupIds": [
"67b687e29ae1f428e7ebe123",
"12s687e29ae1f428e7ebe678"
]
}
Update a task's billable rate
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
projectId
required
string
Default: "##default"
Example: 25b687e29ae1f428e7ebe123
Represents a project identifier across the system.

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
Example: 57a687e29ae1f428e7ebe107
Represents a task identifier across the system.

Request Body schema: application/json
required
amount
required
integer <int32> >= 0
Represents an hourly rate amount as integer.

since	
string
Default: "##default"
Represents a date and time in yyyy-MM-ddThh:mm:ssZ format.

Responses
200 OK
Response Schema: application/json
assigneeId	
string
Deprecated
assigneeIds	
Array of strings unique
Default: "##default"
Represents list of assignee ids for the task.

billable	
boolean
Default: false
Indicates whether a task is billable or not.

budgetEstimate	
integer <int64>
Represents a task budget estimate as long.

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

duration	
string
Default: "##default"
Represents a task duration.

estimate	
string
Default: "##default"
Represents a task duration estimate.

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
Represents task identifier across the system.

name	
string
Default: "##default"
Represents task name.

projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
object (TaskStatus)
Default: "##default"
Represents task status.

One of object
ACTIVE	
string
Enum: "ACTIVE" "DONE" "ALL"
ALL	
string
Enum: "ACTIVE" "DONE" "ALL"
DONE	
string
Enum: "ACTIVE" "DONE" "ALL"
active	
boolean
userGroupIds	
Array of strings unique
Default: "##default"
Represents list of user group ids for the task.


put
/v1/workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/hourly-rate
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/hourly-rate
Request samples
Payload
Content type
application/json

Copy
{
"amount": 20000,
"since": "2020-01-01T00:00:00Z"
}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"assigneeId": "string",
"assigneeIds": [
"45b687e29ae1f428e7ebe123",
"67s687e29ae1f428e7ebe678"
],
"billable": false,
"budgetEstimate": 10000,
"costRate": "##default",
"duration": "PT1H30M",
"estimate": "PT1H30M",
"hourlyRate": "##default",
"id": "57a687e29ae1f428e7ebe107",
"name": "Bugfixing",
"projectId": "25b687e29ae1f428e7ebe123",
"status": "DONE",
"userGroupIds": [
"67b687e29ae1f428e7ebe123",
"12s687e29ae1f428e7ebe678"
]
}
Delete a task from a project
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
taskId
required
string
Default: "##default"
Example: 57a687e29ae1f428e7ebe107
Represents a task identifier across the system.

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
Example: 25b687e29ae1f428e7ebe123
Represents a project identifier across the system.

Responses
200 OK
Response Schema: application/json
assigneeId	
string
Deprecated
assigneeIds	
Array of strings unique
Default: "##default"
Represents list of assignee ids for the task.

billable	
boolean
Default: false
Indicates whether a task is billable or not.

budgetEstimate	
integer <int64>
Represents a task budget estimate as long.

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

duration	
string
Default: "##default"
Represents a task duration.

estimate	
string
Default: "##default"
Represents a task duration estimate.

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
Represents task identifier across the system.

name	
string
Default: "##default"
Represents task name.

projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
object (TaskStatus)
Default: "##default"
Represents task status.

One of object
ACTIVE	
string
Enum: "ACTIVE" "DONE" "ALL"
ALL	
string
Enum: "ACTIVE" "DONE" "ALL"
DONE	
string
Enum: "ACTIVE" "DONE" "ALL"
active	
boolean
userGroupIds	
Array of strings unique
Default: "##default"
Represents list of user group ids for the task.


delete
/v1/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"assigneeId": "string",
"assigneeIds": [
"45b687e29ae1f428e7ebe123",
"67s687e29ae1f428e7ebe678"
],
"billable": false,
"budgetEstimate": 10000,
"costRate": "##default",
"duration": "PT1H30M",
"estimate": "PT1H30M",
"hourlyRate": "##default",
"id": "57a687e29ae1f428e7ebe107",
"name": "Bugfixing",
"projectId": "25b687e29ae1f428e7ebe123",
"status": "DONE",
"userGroupIds": [
"67b687e29ae1f428e7ebe123",
"12s687e29ae1f428e7ebe678"
]
}
Get a task by id
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
taskId
required
string
Default: "##default"
Example: 57a687e29ae1f428e7ebe107
Represents a task identifier across the system.

projectId
required
string
Default: "##default"
Example: 25b687e29ae1f428e7ebe123
Represents a project identifier across the system.

workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Responses
200 OK
Response Schema: application/json
assigneeId	
string
Deprecated
assigneeIds	
Array of strings unique
Default: "##default"
Represents list of assignee ids for the task.

billable	
boolean
Default: false
Indicates whether a task is billable or not.

budgetEstimate	
integer <int64>
Represents a task budget estimate as long.

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

duration	
string
Default: "##default"
Represents a task duration.

estimate	
string
Default: "##default"
Represents a task duration estimate.

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
Represents task identifier across the system.

name	
string
Default: "##default"
Represents task name.

projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
object (TaskStatus)
Default: "##default"
Represents task status.

One of object
ACTIVE	
string
Enum: "ACTIVE" "DONE" "ALL"
ALL	
string
Enum: "ACTIVE" "DONE" "ALL"
DONE	
string
Enum: "ACTIVE" "DONE" "ALL"
active	
boolean
userGroupIds	
Array of strings unique
Default: "##default"
Represents list of user group ids for the task.


get
/v1/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"assigneeId": "string",
"assigneeIds": [
"45b687e29ae1f428e7ebe123",
"67s687e29ae1f428e7ebe678"
],
"billable": false,
"budgetEstimate": 10000,
"costRate": "##default",
"duration": "PT1H30M",
"estimate": "PT1H30M",
"hourlyRate": "##default",
"id": "57a687e29ae1f428e7ebe107",
"name": "Bugfixing",
"projectId": "25b687e29ae1f428e7ebe123",
"status": "DONE",
"userGroupIds": [
"67b687e29ae1f428e7ebe123",
"12s687e29ae1f428e7ebe678"
]
}
Update a task on a project
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
taskId
required
string
Default: "##default"
Example: 57a687e29ae1f428e7ebe107
Represents a task identifier across the system.

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
Example: 25b687e29ae1f428e7ebe123
Represents a project identifier across the system.

query Parameters
contains-assignee	
boolean
Default: true
Flag to set whether task will have assignee or none.

membership-status	
string
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Example: membership-status=ACTIVE
Represents a membership status.

Request Body schema: application/json
required
assigneeId	
string
Deprecated
Default: "##default"
assigneeIds	
Array of strings unique
Default: "##default"
Represents list of assignee ids for the task.

billable	
boolean
Default: false
Indicates whether a task is billable or not.

budgetEstimate	
integer <int64> >= 0
Represents a task budget estimate as integer.

estimate	
string
Default: "##default"
Represents a task duration estimate.

name
required
string [ 1 .. 1000 ] characters
Default: "##default"
Represents task name.

status	
string
Default: "##default"
Enum: "ACTIVE" "DONE" "ALL"
Represents task status.

userGroupIds	
Array of strings unique
Default: "##default"
Represents list of user group ids for the task.

Responses
200 OK
Response Schema: application/json
assigneeId	
string
Deprecated
assigneeIds	
Array of strings unique
Default: "##default"
Represents list of assignee ids for the task.

billable	
boolean
Default: false
Indicates whether a task is billable or not.

budgetEstimate	
integer <int64>
Represents a task budget estimate as long.

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

duration	
string
Default: "##default"
Represents a task duration.

estimate	
string
Default: "##default"
Represents a task duration estimate.

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
Represents task identifier across the system.

name	
string
Default: "##default"
Represents task name.

projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
object (TaskStatus)
Default: "##default"
Represents task status.

userGroupIds	
Array of strings unique
Default: "##default"
Represents list of user group ids for the task.