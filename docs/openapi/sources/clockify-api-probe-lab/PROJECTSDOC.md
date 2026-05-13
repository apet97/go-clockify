Get all projects on a workspace
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
Example: name=Software Development
If provided, you'll get a filtered list of projects that contains the provided string in the project name.

strict-name-search	
boolean
Default: false
Flag to toggle on/off strict search mode. When set to true, search by name will only return projects whose name exactly matches the string value given for the 'name' parameter. When set to false, results will also include projects whose name contain the string value, but could be longer than the string value itself. For example, if there is a project with the name 'applications', and the search value is 'app', setting strict-name-search to true will not return that project in the results, whereas setting it to false will.

archived	
boolean
Default: false
If provided and set to true, you'll only get archived projects. If omitted, you'll get both archived and non-archived projects.

billable	
boolean
Default: false
If provided and set to true, you'll only get billable projects. If omitted, you'll get both billable and non-billable projects.

clients	
Array of strings unique
Default: "##default"
Example: clients=5a0ab5acb07987125438b60f&clients=64c777ddd3fcab07cfbb210c
If provided, you'll get a filtered list of projects that contain clients which match any of the provided ids.

contains-client	
boolean
Default: true
If set to true, you'll get a filtered list of projects that contain clients which match the provided id(s) in 'clients' field. If set to false, you'll get a filtered list of projects which do NOT contain clients that match the provided id(s) in 'clients' field.

client-status	
string
Enum: "ACTIVE" "ARCHIVED" "ALL"
Example: client-status=ACTIVE
Filters projects based on client status provided.

users	
Array of strings unique
Default: "##default"
Example: users=5a0ab5acb07987125438b60f&users=64c777ddd3fcab07cfbb210c
If provided, you'll get a filtered list of projects that contain users which match any of the provided ids.

contains-user	
boolean
Default: true
If set to true, you'll get a filtered list of projects that contain users which match the provided id(s) in 'users' field. If set to false, you'll get a filtered list of projects which do NOT contain users which match the provided id(s) in 'users' field.

user-status	
string
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Example: user-status=ALL
Filters projects based on user status provided.

is-template	
boolean
Default: false
Filters projects based on whether they are used as a template or not.

sort-column	
string
Enum: "ID" "NAME" "CLIENT_NAME" "DURATION" "BUDGET" "PROGRESS"
Example: sort-column=NAME
Sorts the results by the given column/field.

sort-order	
string
Enum: "ASCENDING" "DESCENDING"
Example: sort-order=ASCENDING
Sorting mode.

hydrated	
boolean
Default: false
If set to true, results will contain additional information about the project.

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

access	
string
Enum: "PUBLIC" "PRIVATE"
Example: access=PUBLIC
Valid set of string(s). If provided, you'll get a filtered list of projects that matches the provided access.

expense-limit	
integer <int32>
Default: 20
Example: expense-limit=10
Represents the maximum number of expenses to fetch.

expense-date	
string
Default: "##default"
Example: expense-date=2024-12-31
If provided, you will get expenses dated before the provided value in yyyy-MM-dd format.

userGroups	
Array of strings unique
Default: "##default"
Example: userGroups=5a0ab5acb07987125438b60f&userGroups=64c777ddd3fcab07cfbb210c
If provided, you'll get a filtered list of projects that contain groups which match any of the provided ids.

contains-group	
boolean
Default: true
If set to true, you'll get a filtered list of projects that contain groups which match the provided id(s) in 'userGroups' field. If set to false, you'll get a filtered list of projects which do NOT contain groups which match the provided id(s) in 'userGroups' field.

Responses
200 OK
Response Schema: application/json
Array 
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

budgetEstimate	
object (EstimateWithOptionsDto)
Default: "##default"
Represents a project budget estimate object.

active	
boolean
estimate	
integer <int64>
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Indicates whether estimate includes non-billable or not.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

color	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

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
Represents project duration in milliseconds.

estimate	
object (EstimateDtoV1)
Default: "##default"
Represents a project estimate object.

estimate	
string
Default: "##default"
Represents a task duration estimate.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

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
Represents project identifier across the system.

memberships	
Array of objects (MembershipDtoV1)
Default: "##default"
Represents a list of membership objects.

Array 
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

hourlyRate	
object (HourlyRateDtoV1)
Default: "##default"
Represents an hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

targetId	
string
Default: "##default"
Represents target identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

name	
string
Default: "##default"
Represents a project name.

note	
string
Default: "##default"
Represents project note.

public	
boolean
Default: false
Indicates whether project is public or not.

template	
boolean
Default: false
Indicates whether project is a template or not.

timeEstimate	
object (TimeEstimateDto)
Default: "##default"
Represents a project time estimate object.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/projects
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"archived": false,
"billable": false,
"budgetEstimate": "##default",
"color": "#000000",
"costRate": "##default",
"duration": "60000",
"estimate": "##default",
"hourlyRate": "##default",
"id": "5b641568b07987035750505e",
"memberships": "##default",
"name": "Software Development",
"note": "This is a sample note for the project.",
"public": false,
"template": false,
"timeEstimate": "##default",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Add a new project
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
Indicates whether project is billable or not.

clientId	
string
Default: "##default"
Represents client identifier across the system.

color	
string^#(?:[0-9a-fA-F]{6}){1}$
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

costRate	
object (CostRateRequestV1)
amount
required
integer <int32> >= 0
Represents an amount as integer.

since	
string
Default: "##default"
Represents a date and time in yyyy-MM-ddThh:mm:ssZ format.

estimate	
object (EstimateRequest)
Default: "##default"
Represents an estimate request object.

estimate	
string
Default: "##default"
Represents a time duration in ISO-8601 format.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

hourlyRate	
object (HourlyRateRequestV1)
amount
required
integer <int32> >= 0
Represents an hourly rate amount as integer.

since	
string
Default: "##default"
Represents a date and time in yyyy-MM-ddThh:mm:ssZ format.

isPublic	
boolean
Default: false
Indicates whether project is public or not.

memberships	
Array of objects (MembershipRequest)
Default: "##default"
Represents a list of membership request objects.

Array 
hourlyRate	
object (HourlyRateRequest)
Default: "##default"
Represents an hourly rate request object.

amount
required
integer <int32> >= 0
Represents a cost rate amount as integer.

since	
string
Default: "##default"
Represents a datetime in yyyy-MM-ddThh:mm:ssZ format.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

userId	
string
Default: "##default"
Represents user identifier across the system.

name
required
string [ 2 .. 250 ] characters
Default: "##default"
Represents a project name.

note	
string <= 16384 characters
Default: "##default"
Represents project note.

tasks	
Array of objects (TaskRequest)
Default: "##default"
Represents a list of task request objects.

Array 
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
Flag to set whether task is billable or not

budgetEstimate	
integer <int64> >= 0
costRate	
object (CostRateRequest)
Default: "##default"
Represents a cost rate request object.

amount	
integer <int32> >= 0
Represents an amount as integer.

since	
string
Default: "##default"
Represents a datetime in yyyy-MM-ddThh:mm:ssZ format.

sinceAsInstant	
string <date-time>
estimate	
string
Default: "##default"
Represents a task duration estimate.

hourlyRate	
object (HourlyRateRequest)
Default: "##default"
Represents an hourly rate request object.

amount
required
integer <int32> >= 0
Represents a cost rate amount as integer.

since	
string
Default: "##default"
Represents a datetime in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents task identifier across the system.

name
required
string
Default: "##default"
Represents task name.

projectId	
string
Default: "##default"
Represents project identifier across the system.

status	
string
Default: "##default"
userGroupIds	
Array of strings unique
Default: "##default"
Represents list of user group ids for the task.

Responses
201 Created
Response Schema: application/json
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

budgetEstimate	
object (EstimateWithOptionsDto)
Default: "##default"
Represents a project budget estimate object.

active	
boolean
estimate	
integer <int64>
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Indicates whether estimate includes non-billable or not.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name.

color	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

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
Represents project duration in milliseconds.

estimate	
object (EstimateDtoV1)
Default: "##default"
Represents a project estimate object.

estimate	
string
Default: "##default"
Represents a task duration estimate.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

estimateReset	
object (EstimateResetDto)
Default: "##default"
Represents project estimate reset object

dayOfMonth	
integer <int32>
dayOfWeek	
string
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
hour	
integer <int32>
interval	
string
Enum: "WEEKLY" "MONTHLY" "YEARLY"
month	
string
Enum: "JANUARY" "FEBRUARY" "MARCH" "APRIL" "MAY" "JUNE" "JULY" "AUGUST" "SEPTEMBER" "OCTOBER" "NOVEMBER" "DECEMBER"
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
Represents project identifier across the system.

memberships	
Array of objects (MembershipDtoV1)
Default: "##default"
Represents a list of membership objects.

Array 
costRate	
object (RateDtoV1)
Default: "##default"
Represents hourly rate object.

hourlyRate	
object (HourlyRateDtoV1)
Default: "##default"
Represents an hourly rate object.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

targetId	
string
Default: "##default"
Represents target identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

name	
string
Default: "##default"
Represents a project name.

note	
string
Default: "##default"
Represents project note.

public	
boolean
Default: false
Indicates whether project is public or not.

template	
boolean
Default: false
Indicates whether project is a template or not.

timeEstimate	
object (TimeEstimateDto)
Default: "##default"
Represents a project time estimate object.

active	
boolean
estimate	
string
Default: "##default"
Represents project duration in milliseconds.

includeNonBillable	
boolean
resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/projects
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"billable": false,
"clientId": "9t641568b07987035750704",
"color": "#000000",
"costRate": {
"amount": 20000,
"since": "2020-01-01T00:00:00Z"
},
"estimate": "##default",
"hourlyRate": {
"amount": 20000,
"since": "2020-01-01T00:00:00Z"
},
"isPublic": false,
"memberships": "##default",
"name": "Software Development",
"note": "This is a sample note for the project.",
"tasks": "##default"
}
Response samples
201
Content type
application/json

Copy
{
"archived": false,
"billable": false,
"budgetEstimate": "##default",
"clientId": "9t641568b07987035750704",
"clientName": "Client X",
"color": "#000000",
"costRate": "##default",
"duration": "60000",
"estimate": "##default",
"estimateReset": "##default",
"hourlyRate": "##default",
"id": "5b641568b07987035750505e",
"memberships": "##default",
"name": "Software Development",
"note": "This is a sample note for the project.",
"public": false,
"template": false,
"timeEstimate": "##default",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Create project from a template
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
clientId	
string
Default: "##default"
Represents a client identifier across the system.

color	
string^#(?:[0-9a-fA-F]{6}){1}$
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

isPublic	
boolean
Default: false
Indicates whether the project is public or not.

name
required
string [ 2 .. 250 ] characters
Default: "##default"
Represents a project name.

templateProjectId
required
string non-empty
Default: "##default"
Represents a project identifier across the system.

Responses
200 OK
Response Schema: application/json
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

budgetEstimate	
object (EstimateWithOptionsDto)
Default: "##default"
Represents a project budget estimate object.

active	
boolean
estimate	
integer <int64>
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Indicates whether estimate includes non-billable or not.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name.

color	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

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
Represents project duration in milliseconds.

estimate	
object (EstimateDtoV1)
Default: "##default"
Represents a project estimate object.

estimate	
string
Default: "##default"
Represents a task duration estimate.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

estimateReset	
object (EstimateResetDto)
Default: "##default"
Represents project estimate reset object

dayOfMonth	
integer <int32>
dayOfWeek	
string
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
hour	
integer <int32>
interval	
string
Enum: "WEEKLY" "MONTHLY" "YEARLY"
month	
string
Enum: "JANUARY" "FEBRUARY" "MARCH" "APRIL" "MAY" "JUNE" "JULY" "AUGUST" "SEPTEMBER" "OCTOBER" "NOVEMBER" "DECEMBER"
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
Represents project identifier across the system.

memberships	
Array of objects (MembershipDtoV1)
Default: "##default"
Represents a list of membership objects.

Array 
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

hourlyRate	
object (HourlyRateDtoV1)
Default: "##default"
Represents an hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

targetId	
string
Default: "##default"
Represents target identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

name	
string
Default: "##default"
Represents a project name.

note	
string
Default: "##default"
Represents project note.

public	
boolean
Default: false
Indicates whether project is public or not.

template	
boolean
Default: false
Indicates whether project is a template or not.

timeEstimate	
object (TimeEstimateDto)
Default: "##default"
Represents a project time estimate object.

active	
boolean
estimate	
string
Default: "##default"
Represents project duration in milliseconds.

includeNonBillable	
boolean
resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/projects/from-template
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/from-template
Request samples
Payload
Content type
application/json

Copy
{
"clientId": "9t641568b07987035750704",
"color": "#000000",
"isPublic": false,
"name": "Software Development",
"templateProjectId": "5b641568b07987035750505e"
}
Response samples
200
Content type
application/json

Copy
{
"archived": false,
"billable": false,
"budgetEstimate": "##default",
"clientId": "9t641568b07987035750704",
"clientName": "Client X",
"color": "#000000",
"costRate": "##default",
"duration": "60000",
"estimate": "##default",
"estimateReset": "##default",
"hourlyRate": "##default",
"id": "5b641568b07987035750505e",
"memberships": "##default",
"name": "Software Development",
"note": "This is a sample note for the project.",
"public": false,
"template": false,
"timeEstimate": "##default",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Delete a project from a workspace
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

Responses
200 OK
Response Schema: application/json
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

budgetEstimate	
object (EstimateWithOptionsDto)
Default: "##default"
Represents a project budget estimate object.

active	
boolean
estimate	
integer <int64>
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Indicates whether estimate includes non-billable or not.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name.

color	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

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
Represents project duration in milliseconds.

estimate	
object (EstimateDtoV1)
Default: "##default"
Represents a project estimate object.

estimate	
string
Default: "##default"
Represents a task duration estimate.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

estimateReset	
object (EstimateResetDto)
Default: "##default"
Represents project estimate reset object

dayOfMonth	
integer <int32>
dayOfWeek	
string
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
hour	
integer <int32>
interval	
string
Enum: "WEEKLY" "MONTHLY" "YEARLY"
month	
string
Enum: "JANUARY" "FEBRUARY" "MARCH" "APRIL" "MAY" "JUNE" "JULY" "AUGUST" "SEPTEMBER" "OCTOBER" "NOVEMBER" "DECEMBER"
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
Represents project identifier across the system.

memberships	
Array of objects (MembershipDtoV1)
Default: "##default"
Represents a list of membership objects.

Array 
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

hourlyRate	
object (HourlyRateDtoV1)
Default: "##default"
Represents an hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

targetId	
string
Default: "##default"
Represents target identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

name	
string
Default: "##default"
Represents a project name.

note	
string
Default: "##default"
Represents project note.

public	
boolean
Default: false
Indicates whether project is public or not.

template	
boolean
Default: false
Indicates whether project is a template or not.

timeEstimate	
object (TimeEstimateDto)
Default: "##default"
Represents a project time estimate object.

active	
boolean
estimate	
string
Default: "##default"
Represents project duration in milliseconds.

includeNonBillable	
boolean
resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


delete
/v1/workspaces/{workspaceId}/projects/{projectId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}
Response samples
200
Content type
application/json

Copy
{
"archived": false,
"billable": false,
"budgetEstimate": "##default",
"clientId": "9t641568b07987035750704",
"clientName": "Client X",
"color": "#000000",
"costRate": "##default",
"duration": "60000",
"estimate": "##default",
"estimateReset": "##default",
"hourlyRate": "##default",
"id": "5b641568b07987035750505e",
"memberships": "##default",
"name": "Software Development",
"note": "This is a sample note for the project.",
"public": false,
"template": false,
"timeEstimate": "##default",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Find a project by ID
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
hydrated	
boolean
Default: false
If set to true, results will contain additional information about the project

custom-field-entity-type	
string
Default: "TIMEENTRY"
Example: custom-field-entity-type=TIMEENTRY
If provided, you'll get a filtered list of custom fields that matches the provided string with the custom field entity type.

expense-limit	
integer <int32>
Default: 20
Example: expense-limit=10
Represents the maximum number of expenses to fetch.

expense-date	
string
Default: "##default"
Example: expense-date=2024-12-31
If provided, you will get expenses dated before the provided value in yyyy-MM-dd format.

Responses
200 OK
Response Schema: application/json
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

budgetEstimate	
object (EstimateWithOptionsDto)
Default: "##default"
Represents a project budget estimate object.

active	
boolean
estimate	
integer <int64>
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Indicates whether estimate includes non-billable or not.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

color	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

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
Represents project duration in milliseconds.

estimate	
object (EstimateDtoV1)
Default: "##default"
Represents a project estimate object.

estimate	
string
Default: "##default"
Represents a task duration estimate.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

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
Represents project identifier across the system.

memberships	
Array of objects (MembershipDtoV1)
Default: "##default"
Represents a list of membership objects.

Array 
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

hourlyRate	
object (HourlyRateDtoV1)
Default: "##default"
Represents an hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

targetId	
string
Default: "##default"
Represents target identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

name	
string
Default: "##default"
Represents a project name.

note	
string
Default: "##default"
Represents project note.

public	
boolean
Default: false
Indicates whether project is public or not.

template	
boolean
Default: false
Indicates whether project is a template or not.

timeEstimate	
object (TimeEstimateDto)
Default: "##default"
Represents a project time estimate object.

active	
boolean
estimate	
string
Default: "##default"
Represents project duration in milliseconds.

includeNonBillable	
boolean
resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/projects/{projectId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}
Response samples
200
Content type
application/json

Copy
{
"archived": false,
"billable": false,
"budgetEstimate": "##default",
"color": "#000000",
"costRate": "##default",
"duration": "60000",
"estimate": "##default",
"hourlyRate": "##default",
"id": "5b641568b07987035750505e",
"memberships": "##default",
"name": "Software Development",
"note": "This is a sample note for the project.",
"public": false,
"template": false,
"timeEstimate": "##default",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Update a project on a workspace
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

Request Body schema: application/json
required
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

clientId	
string
Default: "##default"
Represents client identifier across the system.

color	
string^#(?:[0-9a-fA-F]{6}){1}$
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

costRate	
object (CostRateRequestV1)
amount
required
integer <int32> >= 0
Represents an amount as integer.

since	
string
Default: "##default"
Represents a date and time in yyyy-MM-ddThh:mm:ssZ format.

hourlyRate	
object (HourlyRateRequestV1)
amount
required
integer <int32> >= 0
Represents an hourly rate amount as integer.

since	
string
Default: "##default"
Represents a date and time in yyyy-MM-ddThh:mm:ssZ format.

isPublic	
boolean
Default: false
Indicates whether project is public or not.

name	
string [ 2 .. 250 ] characters
Default: "##default"
Represents a project name.

note	
string <= 16384 characters
Default: "##default"
Represents project note.

Responses
200 OK
Response Schema: application/json
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

budgetEstimate	
object (EstimateWithOptionsDto)
Default: "##default"
Represents a project budget estimate object.

active	
boolean
estimate	
integer <int64>
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Indicates whether estimate includes non-billable or not.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name.

color	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

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
Represents project duration in milliseconds.

estimate	
object (EstimateDtoV1)
Default: "##default"
Represents a project estimate object.

estimate	
string
Default: "##default"
Represents a task duration estimate.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

estimateReset	
object (EstimateResetDto)
Default: "##default"
Represents project estimate reset object

dayOfMonth	
integer <int32>
dayOfWeek	
string
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
hour	
integer <int32>
interval	
string
Enum: "WEEKLY" "MONTHLY" "YEARLY"
month	
string
Enum: "JANUARY" "FEBRUARY" "MARCH" "APRIL" "MAY" "JUNE" "JULY" "AUGUST" "SEPTEMBER" "OCTOBER" "NOVEMBER" "DECEMBER"
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
Represents project identifier across the system.

memberships	
Array of objects (MembershipDtoV1)
Default: "##default"
Represents a list of membership objects.

Array 
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

hourlyRate	
object (HourlyRateDtoV1)
Default: "##default"
Represents an hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

targetId	
string
Default: "##default"
Represents target identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

name	
string
Default: "##default"
Represents a project name.

note	
string
Default: "##default"
Represents project note.

public	
boolean
Default: false
Indicates whether project is public or not.

template	
boolean
Default: false
Indicates whether project is a template or not.

timeEstimate	
object (TimeEstimateDto)
Default: "##default"
Represents a project time estimate object.

active	
boolean
estimate	
string
Default: "##default"
Represents project duration in milliseconds.

includeNonBillable	
boolean
resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/projects/{projectId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"archived": false,
"billable": false,
"clientId": "9t641568b07987035750704",
"color": "#000000",
"costRate": {
"amount": 20000,
"since": "2020-01-01T00:00:00Z"
},
"hourlyRate": {
"amount": 20000,
"since": "2020-01-01T00:00:00Z"
},
"isPublic": false,
"name": "Software Development",
"note": "This is a sample note for the project."
}
Response samples
200
Content type
application/json

Copy
{
"archived": false,
"billable": false,
"budgetEstimate": "##default",
"clientId": "9t641568b07987035750704",
"clientName": "Client X",
"color": "#000000",
"costRate": "##default",
"duration": "60000",
"estimate": "##default",
"estimateReset": "##default",
"hourlyRate": "##default",
"id": "5b641568b07987035750505e",
"memberships": "##default",
"name": "Software Development",
"note": "This is a sample note for the project.",
"public": false,
"template": false,
"timeEstimate": "##default",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Update project estimate
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

Request Body schema: application/json
required
budgetEstimate	
object (EstimateWithOptionsRequest)
Default: "##default"
Represents estimate with options request object.

active	
boolean
Default: false
Flag whether to set estimate as active or not.

estimate	
integer <int64> >= 0
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Flag whether to include billable expenses.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

estimateReset	
object (EstimateResetRequest)
Default: "##default"
Represents estimate reset request object.

active	
boolean
dayOfMonth	
integer <int32> [ 1 .. 31 ]
Represents a day of the month.

dayOfWeek	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

hour	
integer <int32> [ 0 .. 23 ]
Represents an hour of the day in 24 hour time format.

interval	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

isActive	
boolean
month	
string
Default: "##default"
Enum: "JANUARY" "FEBRUARY" "MARCH" "APRIL" "MAY" "JUNE" "JULY" "AUGUST" "SEPTEMBER" "OCTOBER" "NOVEMBER" "DECEMBER"
Represents a month enum.

timeEstimate	
object (TimeEstimateRequest)
Default: "##default"
Represents project time estimate request object.

active	
boolean
Default: false
Flag whether to include only active or inactive estimates.

estimate	
string
Default: "##default"
Represents a time duration in ISO-8601 format.

includeNonBillable	
boolean
Default: false
Flag whether to include non-billable expenses.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

Responses
200 OK
Response Schema: application/json
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

budgetEstimate	
object (EstimateWithOptionsDto)
Default: "##default"
Represents a project budget estimate object.

active	
boolean
estimate	
integer <int64>
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Indicates whether estimate includes non-billable or not.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name.

color	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

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
Represents project duration in milliseconds.

estimate	
object (EstimateDtoV1)
Default: "##default"
Represents a project estimate object.

estimate	
string
Default: "##default"
Represents a task duration estimate.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

estimateReset	
object (EstimateResetDto)
Default: "##default"
Represents project estimate reset object

dayOfMonth	
integer <int32>
dayOfWeek	
string
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
hour	
integer <int32>
interval	
string
Enum: "WEEKLY" "MONTHLY" "YEARLY"
month	
string
Enum: "JANUARY" "FEBRUARY" "MARCH" "APRIL" "MAY" "JUNE" "JULY" "AUGUST" "SEPTEMBER" "OCTOBER" "NOVEMBER" "DECEMBER"
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
Represents project identifier across the system.

memberships	
Array of objects (MembershipDtoV1)
Default: "##default"
Represents a list of membership objects.

Array 
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

hourlyRate	
object (HourlyRateDtoV1)
Default: "##default"
Represents an hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

targetId	
string
Default: "##default"
Represents target identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

name	
string
Default: "##default"
Represents a project name.

note	
string
Default: "##default"
Represents project note.

public	
boolean
Default: false
Indicates whether project is public or not.

template	
boolean
Default: false
Indicates whether project is a template or not.

timeEstimate	
object (TimeEstimateDto)
Default: "##default"
Represents a project time estimate object.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


patch
/v1/workspaces/{workspaceId}/projects/{projectId}/estimate
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/estimate
Request samples
Payload
Content type
application/json

Copy
{
"budgetEstimate": "##default",
"estimateReset": "##default",
"timeEstimate": "##default"
}
Response samples
200
Content type
application/json

Copy
{
"archived": false,
"billable": false,
"budgetEstimate": "##default",
"clientId": "9t641568b07987035750704",
"clientName": "Client X",
"color": "#000000",
"costRate": "##default",
"duration": "60000",
"estimate": "##default",
"estimateReset": "##default",
"hourlyRate": "##default",
"id": "5b641568b07987035750505e",
"memberships": "##default",
"name": "Software Development",
"note": "This is a sample note for the project.",
"public": false,
"template": false,
"timeEstimate": "##default",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Update project memberships
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

Request Body schema: application/json
required
memberships
required
Array of objects (UserIdWithRatesRequest)
Default: "##default"
Represents a list of users with id and rates request objects.

Array 
costRate	
object (CostRateRequestV1)
amount
required
integer <int32> >= 0
Represents an amount as integer.

since	
string
Default: "##default"
Represents a date and time in yyyy-MM-ddThh:mm:ssZ format.

hourlyRate	
object (HourlyRateRequestV1)
amount
required
integer <int32> >= 0
Represents an hourly rate amount as integer.

since	
string
Default: "##default"
Represents a date and time in yyyy-MM-ddThh:mm:ssZ format.

userId
required
string
Default: "##default"
Represents user identifier across the system.

userGroups	
object (UserGroupIdsSchema)
Default: "##default"
Provide list with user group ids and corresponding status.

contains	
string
Default: "##default"
Enum: "CONTAINS" "DOES_NOT_CONTAIN"
ids	
Array of strings unique
Default: "##default"
Represents ids upon which filtering is performed.

status	
string
Default: "##default"
Enum: "ALL" "ACTIVE" "INACTIVE"
Represents user status.

Responses
200 OK
Response Schema: application/json
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

budgetEstimate	
object (EstimateWithOptionsDto)
Default: "##default"
Represents a project budget estimate object.

active	
boolean
estimate	
integer <int64>
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Indicates whether estimate includes non-billable or not.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name.

color	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

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
Represents project duration in milliseconds.

estimate	
object (EstimateDtoV1)
Default: "##default"
Represents a project estimate object.

estimate	
string
Default: "##default"
Represents a task duration estimate.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

estimateReset	
object (EstimateResetDto)
Default: "##default"
Represents project estimate reset object

dayOfMonth	
integer <int32>
dayOfWeek	
string
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
hour	
integer <int32>
interval	
string
Enum: "WEEKLY" "MONTHLY" "YEARLY"
month	
string
Enum: "JANUARY" "FEBRUARY" "MARCH" "APRIL" "MAY" "JUNE" "JULY" "AUGUST" "SEPTEMBER" "OCTOBER" "NOVEMBER" "DECEMBER"
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
Represents project identifier across the system.

memberships	
Array of objects (MembershipDtoV1)
Default: "##default"
Represents a list of membership objects.

Array 
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

hourlyRate	
object (HourlyRateDtoV1)
Default: "##default"
Represents an hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

targetId	
string
Default: "##default"
Represents target identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

name	
string
Default: "##default"
Represents a project name.

note	
string
Default: "##default"
Represents project note.

public	
boolean
Default: false
Indicates whether project is public or not.

template	
boolean
Default: false
Indicates whether project is a template or not.

timeEstimate	
object (TimeEstimateDto)
Default: "##default"
Represents a project time estimate object.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


patch
/v1/workspaces/{workspaceId}/projects/{projectId}/memberships
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/memberships
Request samples
Payload
Content type
application/json

Copy
{
"memberships": "##default",
"userGroups": "##default"
}
Response samples
200
Content type
application/json

Copy
{
"archived": false,
"billable": false,
"budgetEstimate": "##default",
"clientId": "9t641568b07987035750704",
"clientName": "Client X",
"color": "#000000",
"costRate": "##default",
"duration": "60000",
"estimate": "##default",
"estimateReset": "##default",
"hourlyRate": "##default",
"id": "5b641568b07987035750505e",
"memberships": "##default",
"name": "Software Development",
"note": "This is a sample note for the project.",
"public": false,
"template": false,
"timeEstimate": "##default",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Assign/remove users to/from the project
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

Request Body schema: application/json
required
remove	
boolean
Default: false
Setting this flag to 'true' will remove the given users from the project.

userGroups	
object (UserGroupIdsSchema)
Default: "##default"
Provide list with user group ids and corresponding status.

contains	
string
Default: "##default"
Enum: "CONTAINS" "DOES_NOT_CONTAIN"
ids	
Array of strings unique
Default: "##default"
Represents ids upon which filtering is performed.

status	
string
Default: "##default"
Enum: "ALL" "ACTIVE" "INACTIVE"
Represents user status.

userIds	
Array of strings
Default: "##default"
Represents array of user ids which should be added/removed.

Responses
200 OK
Response Schema: */*
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

budgetEstimate	
object (EstimateWithOptionsDto)
Default: "##default"
Represents a project budget estimate object.

active	
boolean
estimate	
integer <int64>
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Indicates whether estimate includes non-billable or not.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name.

color	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

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
Represents project duration in milliseconds.

estimate	
object (EstimateDtoV1)
Default: "##default"
Represents a project estimate object.

estimate	
string
Default: "##default"
Represents a task duration estimate.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

estimateReset	
object (EstimateResetDto)
Default: "##default"
Represents project estimate reset object

dayOfMonth	
integer <int32>
dayOfWeek	
string
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
hour	
integer <int32>
interval	
string
Enum: "WEEKLY" "MONTHLY" "YEARLY"
month	
string
Enum: "JANUARY" "FEBRUARY" "MARCH" "APRIL" "MAY" "JUNE" "JULY" "AUGUST" "SEPTEMBER" "OCTOBER" "NOVEMBER" "DECEMBER"
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
Represents project identifier across the system.

memberships	
Array of objects (MembershipDtoV1)
Default: "##default"
Represents a list of membership objects.

Array 
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

hourlyRate	
object (HourlyRateDtoV1)
Default: "##default"
Represents an hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

targetId	
string
Default: "##default"
Represents target identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

name	
string
Default: "##default"
Represents a project name.

note	
string
Default: "##default"
Represents project note.

public	
boolean
Default: false
Indicates whether project is public or not.

template	
boolean
Default: false
Indicates whether project is a template or not.

timeEstimate	
object (TimeEstimateDto)
Default: "##default"
Represents a project time estimate object.

active	
boolean
estimate	
string
Default: "##default"
Represents project duration in milliseconds.

includeNonBillable	
boolean
resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/projects/{projectId}/memberships
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/memberships
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"remove": false,
"userGroups": "##default",
"userIds": [
"45b687e29ae1f428e7ebe123",
"67s687e29ae1f428e7ebe678"
]
}
Update a project template
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

Request Body schema: application/json
required
isTemplate	
boolean
Default: false
Indicates whether project is a template or not.

Responses
200 OK
Response Schema: application/json
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

budgetEstimate	
object (EstimateWithOptionsDto)
Default: "##default"
Represents a project budget estimate object.

active	
boolean
estimate	
integer <int64>
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Indicates whether estimate includes non-billable or not.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name.

color	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

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
Represents project duration in milliseconds.

estimate	
object (EstimateDtoV1)
Default: "##default"
Represents a project estimate object.

estimate	
string
Default: "##default"
Represents a task duration estimate.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

estimateReset	
object (EstimateResetDto)
Default: "##default"
Represents project estimate reset object

dayOfMonth	
integer <int32>
dayOfWeek	
string
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
hour	
integer <int32>
interval	
string
Enum: "WEEKLY" "MONTHLY" "YEARLY"
month	
string
Enum: "JANUARY" "FEBRUARY" "MARCH" "APRIL" "MAY" "JUNE" "JULY" "AUGUST" "SEPTEMBER" "OCTOBER" "NOVEMBER" "DECEMBER"
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
Represents project identifier across the system.

memberships	
Array of objects (MembershipDtoV1)
Default: "##default"
Represents a list of membership objects.

Array 
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

hourlyRate	
object (HourlyRateDtoV1)
Default: "##default"
Represents an hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

targetId	
string
Default: "##default"
Represents target identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

name	
string
Default: "##default"
Represents a project name.

note	
string
Default: "##default"
Represents project note.

public	
boolean
Default: false
Indicates whether project is public or not.

template	
boolean
Default: false
Indicates whether project is a template or not.

timeEstimate	
object (TimeEstimateDto)
Default: "##default"
Represents a project time estimate object.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


patch
/v1/workspaces/{workspaceId}/projects/{projectId}/template
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/template
Request samples
Payload
Content type
application/json

Copy
{
"isTemplate": false
}
Response samples
200
Content type
application/json

Copy
{
"archived": false,
"billable": false,
"budgetEstimate": "##default",
"clientId": "9t641568b07987035750704",
"clientName": "Client X",
"color": "#000000",
"costRate": "##default",
"duration": "60000",
"estimate": "##default",
"estimateReset": "##default",
"hourlyRate": "##default",
"id": "5b641568b07987035750505e",
"memberships": "##default",
"name": "Software Development",
"note": "This is a sample note for the project.",
"public": false,
"template": false,
"timeEstimate": "##default",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Update project user's cost rate
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

userId
required
string
Default: "##default"
Example: 4a0ab5acb07987125438b60f
Represents a user identifier across the system.

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
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

budgetEstimate	
object (EstimateWithOptionsDto)
Default: "##default"
Represents a project budget estimate object.

active	
boolean
estimate	
integer <int64>
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Indicates whether estimate includes non-billable or not.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name.

color	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

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
Represents project duration in milliseconds.

estimate	
object (EstimateDtoV1)
Default: "##default"
Represents a project estimate object.

estimate	
string
Default: "##default"
Represents a task duration estimate.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

estimateReset	
object (EstimateResetDto)
Default: "##default"
Represents project estimate reset object

dayOfMonth	
integer <int32>
dayOfWeek	
string
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
hour	
integer <int32>
interval	
string
Enum: "WEEKLY" "MONTHLY" "YEARLY"
month	
string
Enum: "JANUARY" "FEBRUARY" "MARCH" "APRIL" "MAY" "JUNE" "JULY" "AUGUST" "SEPTEMBER" "OCTOBER" "NOVEMBER" "DECEMBER"
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
Represents project identifier across the system.

memberships	
Array of objects (MembershipDtoV1)
Default: "##default"
Represents a list of membership objects.

Array 
costRate	
object (RateDtoV1)
Default: "##default"
Represents hourly rate object.

hourlyRate	
object (HourlyRateDtoV1)
Default: "##default"
Represents an hourly rate object.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

targetId	
string
Default: "##default"
Represents target identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

name	
string
Default: "##default"
Represents a project name.

note	
string
Default: "##default"
Represents project note.

public	
boolean
Default: false
Indicates whether project is public or not.

template	
boolean
Default: false
Indicates whether project is a template or not.

timeEstimate	
object (TimeEstimateDto)
Default: "##default"
Represents a project time estimate object.

active	
boolean
estimate	
string
Default: "##default"
Represents project duration in milliseconds.

includeNonBillable	
boolean
resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/projects/{projectId}/users/{userId}/cost-rate
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/users/{userId}/cost-rate
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
{
"archived": false,
"billable": false,
"budgetEstimate": "##default",
"clientId": "9t641568b07987035750704",
"clientName": "Client X",
"color": "#000000",
"costRate": "##default",
"duration": "60000",
"estimate": "##default",
"estimateReset": "##default",
"hourlyRate": "##default",
"id": "5b641568b07987035750505e",
"memberships": "##default",
"name": "Software Development",
"note": "This is a sample note for the project.",
"public": false,
"template": false,
"timeEstimate": "##default",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Update a project user's billable rate
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

userId
required
string
Default: "##default"
Example: 4a0ab5acb07987125438b60f
Represents a user identifier across the system.

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
archived	
boolean
Default: false
Indicates whether project is archived or not.

billable	
boolean
Default: false
Indicates whether project is billable or not.

budgetEstimate	
object (EstimateWithOptionsDto)
Default: "##default"
Represents a project budget estimate object.

active	
boolean
estimate	
integer <int64>
Represents an estimate as long.

includeExpenses	
boolean
Default: false
Indicates whether estimate includes non-billable or not.

resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents client name.

color	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

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
Represents project duration in milliseconds.

estimate	
object (EstimateDtoV1)
Default: "##default"
Represents a project estimate object.

estimate	
string
Default: "##default"
Represents a task duration estimate.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

estimateReset	
object (EstimateResetDto)
Default: "##default"
Represents project estimate reset object

dayOfMonth	
integer <int32>
dayOfWeek	
string
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
hour	
integer <int32>
interval	
string
Enum: "WEEKLY" "MONTHLY" "YEARLY"
month	
string
Enum: "JANUARY" "FEBRUARY" "MARCH" "APRIL" "MAY" "JUNE" "JULY" "AUGUST" "SEPTEMBER" "OCTOBER" "NOVEMBER" "DECEMBER"
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
Represents project identifier across the system.

memberships	
Array of objects (MembershipDtoV1)
Default: "##default"
Represents a list of membership objects.

Array 
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

hourlyRate	
object (HourlyRateDtoV1)
Default: "##default"
Represents an hourly rate object.

amount	
integer <int32>
Represents an amount as integer.

currency	
string
Default: "##default"
Represents a currency.

membershipStatus	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Represents a membership status enum.

membershipType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "USERGROUP"
Represents membership type enum.

targetId	
string
Default: "##default"
Represents target identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

name	
string
Default: "##default"
Represents a project name.

note	
string
Default: "##default"
Represents project note.

public	
boolean
Default: false
Indicates whether project is public or not.

template	
boolean
Default: false
Indicates whether project is a template or not.

timeEstimate	
object (TimeEstimateDto)
Default: "##default"
Represents a project time estimate object.

active	
boolean
estimate	
string
Default: "##default"
Represents project duration in milliseconds.

includeNonBillable	
boolean
resetOption	
string
Default: "##default"
Enum: "WEEKLY" "MONTHLY" "YEARLY"
Represents a reset option enum.

type	
string
Default: "##default"
Enum: "AUTO" "MANUAL"
Represents an estimate type enum.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/projects/{projectId}/users/{userId}/hourly-rate
https://api.clockify.me/api/v1/workspaces/{workspaceId}/projects/{projectId}/users/{userId}/hourly-rate
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
{
"archived": false,
"billable": false,
"budgetEstimate": "##default",
"clientId": "9t641568b07987035750704",
"clientName": "Client X",
"color": "#000000",
"costRate": "##default",
"duration": "60000",
"estimate": "##default",
"estimateReset": "##default",
"hourlyRate": "##default",
"id": "5b641568b07987035750505e",
"memberships": "##default",
"name": "Software Development",
"note": "This is a sample note for the project.",
"public": false,
"template": false,
"timeEstimate": "##default",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
