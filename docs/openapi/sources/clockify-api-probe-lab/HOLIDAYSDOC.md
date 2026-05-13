Get holidays on a workspace
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

query Parameters
assigned-to	
string
Default: "##default"
Example: assigned-to=60f924bafdaf031696ec6218
If provided, you'll get a filtered list of holidays assigned to user.

Responses
200 OK
Response Schema: application/json
Array 
automaticTimeEntryCreation	
boolean
Default: false
Indicates that time entries will be automatically created for this holiday.

datePeriod	
object (DatePeriod)
Default: "##default"
Represents startDate and endDate of the holiday. Date is in format yyyy-mm-dd

endDate	
string <date>
startDate	
string <date>
everyoneIncludingNew	
boolean
Default: false
Indicates whether the holiday is shown to new users.

id	
string
Default: "##default"
Represents holiday identifier across the system.

name	
string
Default: "##default"
Represents the name of the holiday.

occursAnnually	
boolean
Default: false
Indicates whether the holiday occurs annually.

projectId	
string
Default: "##default"
Represents projectId for automatic time entry creation.

taskId	
string
Default: "##default"
Represents taskId for automatic time entry creation.

userGroupIds	
Array of strings unique
Default: "##default"
Indicates which user groups are included.

userIds	
Array of strings unique
Default: "##default"
Indicates which users are included.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/holidays
https://api.clockify.me/api/v1/workspaces/{workspaceId}/holidays
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"automaticTimeEntryCreation": false,
"datePeriod": "##default",
"everyoneIncludingNew": false,
"id": "5b715612b079875110791111",
"name": "New Year's Day",
"occursAnnually": true,
"projectId": "65b36d3c525e243c48f9150f",
"taskId": "65b36d46fa3df8607e42d21a",
"userGroupIds": [
"5b715612b079875110791342",
"5b715612b079875110791324",
"5b715612b079875110793142"
],
"userIds": [
"5b715612b079875110791432",
"5b715612b079875110791234"
],
"workspaceId": "5b715612b079875110792222"
}
]
Create a holiday
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

Request Body schema: application/json
required
automaticTimeEntryCreation	
object (AutomaticTimeEntryCreationRequest)
Default: "##default"
Provides automatic time entry creation settings.

defaultEntities
required
object (DefaultEntitiesRequest)
Default: "##default"
Provides information about default project and task for automatically created time entries.

projectId	
string
Default: "##default"
Default project for automatically created time entries

taskId	
string
Default: "##default"
Default task for automatically created time entries

enabled	
boolean
Default: false
Indicates that automatic time entry creation is enabled.

color	
string^#(?:[0-9a-fA-F]{6}){1}$
Default: "##default"
Provide color in format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

datePeriod
required
object (DatePeriodRequest)
Default: "##default"
Provide startDate and endDate for the holiday.

endDate
required
string non-empty
Default: "##default"
yyyy-MM-dd format date

startDate
required
string non-empty
Default: "##default"
yyyy-MM-dd format date

everyoneIncludingNew	
boolean
Default: false
Indicates whether the holiday is shown to new users.

name
required
string [ 2 .. 100 ] characters
Default: "##default"
Provide the name of the holiday.

occursAnnually	
boolean
Default: false
Indicates whether the holiday occurs annually.

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

users	
object (UserIdsSchema)
Default: "##default"
Provide list with user ids and corresponding status.

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
automaticTimeEntryCreation	
boolean
Default: false
Indicates that time entries will be automatically created for this holiday.

datePeriod	
object (DatePeriod)
Default: "##default"
Represents startDate and endDate of the holiday. Date is in format yyyy-mm-dd

endDate	
string <date>
startDate	
string <date>
everyoneIncludingNew	
boolean
Default: false
Indicates whether the holiday is shown to new users.

id	
string
Default: "##default"
Represents holiday identifier across the system.

name	
string
Default: "##default"
Represents the name of the holiday.

occursAnnually	
boolean
Default: false
Indicates whether the holiday occurs annually.

projectId	
string
Default: "##default"
Represents projectId for automatic time entry creation.

taskId	
string
Default: "##default"
Represents taskId for automatic time entry creation.

userGroupIds	
Array of strings unique
Default: "##default"
Indicates which user groups are included.

userIds	
Array of strings unique
Default: "##default"
Indicates which users are included.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/holidays
https://api.clockify.me/api/v1/workspaces/{workspaceId}/holidays
Request samples
Payload
Content type
application/json

Copy
{
"automaticTimeEntryCreation": "##default",
"color": "#8BC34A",
"datePeriod": "##default",
"everyoneIncludingNew": true,
"name": "Labour Day",
"occursAnnually": true,
"userGroups": "##default",
"users": "##default"
}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"automaticTimeEntryCreation": false,
"datePeriod": "##default",
"everyoneIncludingNew": false,
"id": "5b715612b079875110791111",
"name": "New Year's Day",
"occursAnnually": true,
"projectId": "65b36d3c525e243c48f9150f",
"taskId": "65b36d46fa3df8607e42d21a",
"userGroupIds": [
"5b715612b079875110791342",
"5b715612b079875110791324",
"5b715612b079875110793142"
],
"userIds": [
"5b715612b079875110791432",
"5b715612b079875110791234"
],
"workspaceId": "5b715612b079875110792222"
}
Get holidays in a specific period
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

query Parameters
assigned-to
required
string
Default: "##default"
Example: assigned-to=60f924bafdaf031696ec6218
Filter list of holidays assigned to user.

start
required
string
Default: "##default"
Example: start=2022-12-03T10:59:59.999Z
Filter list of holidays starting from start date. Expected date format yyyy-MM-ddThh:mm:ssZ

end
required
string
Default: "##default"
Example: end=2022-12-05T23:59:59.999Z
Filter list of holidays ending by end date. Expected date format yyyy-MM-ddThh:mm:ssZ

Responses
200 OK
Response Schema: application/json
Array 
automaticTimeEntryCreation	
boolean
Default: false
Indicates that time entries will be automatically created for this holiday.

datePeriod	
object (DatePeriod)
Default: "##default"
Represents startDate and endDate of the holiday. Date is in format yyyy-mm-dd

endDate	
string <date>
startDate	
string <date>
everyoneIncludingNew	
boolean
Default: false
Indicates whether the holiday is shown to new users.

id	
string
Default: "##default"
Represents holiday identifier across the system.

name	
string
Default: "##default"
Represents the name of the holiday.

occursAnnually	
boolean
Default: false
Indicates whether the holiday occurs annually.

projectId	
string
Default: "##default"
Represents projectId for automatic time entry creation.

taskId	
string
Default: "##default"
Represents taskId for automatic time entry creation.

userGroupIds	
Array of strings unique
Default: "##default"
Indicates which user groups are included.

userIds	
Array of strings unique
Default: "##default"
Indicates which users are included.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/holidays/in-period
https://api.clockify.me/api/v1/workspaces/{workspaceId}/holidays/in-period
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"automaticTimeEntryCreation": false,
"datePeriod": "##default",
"everyoneIncludingNew": false,
"id": "5b715612b079875110791111",
"name": "New Year's Day",
"occursAnnually": true,
"projectId": "65b36d3c525e243c48f9150f",
"taskId": "65b36d46fa3df8607e42d21a",
"userGroupIds": [
"5b715612b079875110791342",
"5b715612b079875110791324",
"5b715612b079875110793142"
],
"userIds": [
"5b715612b079875110791432",
"5b715612b079875110791234"
],
"workspaceId": "5b715612b079875110792222"
}
]
Delete a holiday
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

holidayId
required
string
Default: "##default"
Example: 60f927920658241e3cf35e02
Represents a holiday identifier across the system.

Responses
200 OK
Response Schema: application/json
automaticTimeEntryCreation	
object (AutomaticTimeEntryCreationDto)
Default: "##default"
Represents automatic time entry creation settings.

defaultEntities	
object (DefaultEntitiesDto)
projectId	
string
taskId	
string
enabled	
boolean
color	
string
Default: "##default"
Provide color in format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

datePeriod	
object (DatePeriod)
Default: "##default"
Represents startDate and endDate of the holiday. Date is in format yyyy-mm-dd

endDate	
string <date>
startDate	
string <date>
everyoneIncludingNew	
boolean
Default: false
Indicates whether the holiday is shown to new users.

id	
string
Default: "##default"
Represents holiday identifier across the system.

name	
string
Default: "##default"
Represents the name of the holiday.

occursAnnually	
boolean
Default: false
Indicates whether the holiday occurs annually.

userGroupIds	
Array of strings unique
Default: "##default"
Indicates which user groups are included.

userGroups	
Array of objects (EntityIdNameDto)
Default: "##default"
Contains names of user groups that are assigned to holiday.

Array 
id	
string
name	
string
userIds	
Array of strings unique
Default: "##default"
Indicates which users are included.

users	
Array of objects (EntityIdNameDto)
Default: "##default"
Contains names of users that are assigned to holiday.

Array 
id	
string
name	
string
workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


delete
/v1/workspaces/{workspaceId}/holidays/{holidayId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/holidays/{holidayId}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"automaticTimeEntryCreation": "##default",
"color": "#8BC34A",
"datePeriod": "##default",
"everyoneIncludingNew": false,
"id": "5b715612b079875110791111",
"name": "New Year's Day",
"occursAnnually": true,
"userGroupIds": [
"5b715612b079875110791342",
"5b715612b079875110791324",
"5b715612b079875110793142"
],
"userGroups": "##default",
"userIds": [
"5b715612b079875110791432",
"5b715612b079875110791234"
],
"users": "##default",
"workspaceId": "5b715612b079875110792222"
}
Update a holiday
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

holidayId
required
string
Default: "##default"
Example: 60f927920658241e3cf35e02
Represents a holiday identifier across the system.

Request Body schema: application/json
required
automaticTimeEntryCreation	
object (AutomaticTimeEntryCreationRequest)
Default: "##default"
Provides automatic time entry creation settings.

defaultEntities
required
object (DefaultEntitiesRequest)
Default: "##default"
Provides information about default project and task for automatically created time entries.

projectId	
string
Default: "##default"
Default project for automatically created time entries

taskId	
string
Default: "##default"
Default task for automatically created time entries

enabled	
boolean
Default: false
Indicates that automatic time entry creation is enabled.

color	
string^#(?:[0-9a-fA-F]{6}){1}$
Default: "##default"
Provide color in format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

datePeriod
required
object (DatePeriodRequest)
Default: "##default"
Provide startDate and endDate for the holiday.

endDate
required
string non-empty
Default: "##default"
yyyy-MM-dd format date

startDate
required
string non-empty
Default: "##default"
yyyy-MM-dd format date

everyoneIncludingNew	
boolean
Default: false
Indicates whether the holiday is shown to new users.

name
required
string non-empty
Default: "##default"
Provide the name you would like to use for updating the holiday.

occursAnnually
required
boolean
Default: false
Indicates whether the holiday occurs annually.

userGroups	
object (ContainsUserGroupFilterRequest)
Default: "##default"
Provide list with user group ids and corresponding status.

contains	
string
Default: "##default"
Enum: "CONTAINS" "DOES_NOT_CONTAIN" "CONTAINS_ONLY"
Filter type.

ids	
Array of strings unique
Default: "##default"
Represents a list of filter identifiers.

status	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Filters entities by status.

users	
object (ContainsUsersFilterRequestForHoliday)
Default: "##default"
Provide list with users ids and corresponding status.

contains	
string
Default: "##default"
Enum: "CONTAINS" "DOES_NOT_CONTAIN" "CONTAINS_ONLY"
Filter type.

ids	
Array of strings unique
Default: "##default"
Represents a list of filter identifiers.

status	
string
Default: "##default"
Enum: "ALL" "ACTIVE" "INACTIVE"
Filters entities by status.

statuses	
Array of strings
Responses
200 OK
Response Schema: application/json
automaticTimeEntryCreation	
boolean
Default: false
Indicates that time entries will be automatically created for this holiday.

datePeriod	
object (DatePeriod)
Default: "##default"
Represents startDate and endDate of the holiday. Date is in format yyyy-mm-dd

endDate	
string <date>
startDate	
string <date>
everyoneIncludingNew	
boolean
Default: false
Indicates whether the holiday is shown to new users.

id	
string
Default: "##default"
Represents holiday identifier across the system.

name	
string
Default: "##default"
Represents the name of the holiday.

occursAnnually	
boolean
Default: false
Indicates whether the holiday occurs annually.

projectId	
string
Default: "##default"
Represents projectId for automatic time entry creation.

taskId	
string
Default: "##default"
Represents taskId for automatic time entry creation.

userGroupIds	
Array of strings unique
Default: "##default"
Indicates which user groups are included.

userIds	
Array of strings unique
Default: "##default"
Indicates which users are included.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/holidays/{holidayId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/holidays/{holidayId}
Request samples
Payload
Content type
application/json

Copy
{
"automaticTimeEntryCreation": "##default",
"color": "#8BC34A",
"datePeriod": "##default",
"everyoneIncludingNew": false,
"name": "New Year's Day",
"occursAnnually": true,
"userGroups": "##default",
"users": "##default"
}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"automaticTimeEntryCreation": false,
"datePeriod": "##default",
"everyoneIncludingNew": false,
"id": "5b715612b079875110791111",
"name": "New Year's Day",
"occursAnnually": true,
"projectId": "65b36d3c525e243c48f9150f",
"taskId": "65b36d46fa3df8607e42d21a",
"userGroupIds": [
"5b715612b079875110791342",
"5b715612b079875110791324",
"5b715612b079875110793142"
],
"userIds": [
"5b715612b079875110791432",
"5b715612b079875110791234"
],
"workspaceId": "5b715612b079875110792222"
}
