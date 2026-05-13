Get policies on a workspace
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
page	
string <= 1000
Default: "##default"
Page number.

page-size	
integer <int32> [ 1 .. 200 ]
Default: 50
Example: page-size=50
Page size.

name	
string
Default: "##default"
Example: name=Holidays
If provided, you'll get a filtered list of policies that contain the provided string in their name.

status	
string
Enum: "ACTIVE" "ARCHIVED" "ALL"
Example: status=ACTIVE
If provided, you'll get a filtered list of policies with the corresponding status.

sort-column	
string
Default: "DEFAULT_SORT"
sort-order	
string
Default: "ASCENDING"
Responses
200 OK
Response Schema: application/json
Array 
allowHalfDay	
boolean
Default: false
Indicates whether the half day is allowed.

allowNegativeBalance	
boolean
Default: false
Indicates whether the negative balance is allowed.

approve	
object (PolicyApprovalDto)
Default: "##default"
Represents approval settings.

requiresApproval	
boolean
Default: false
Indicates whether it requires approval

specificMembers	
boolean
Default: false
Indicates whether it requires specific members

teamManagers	
boolean
Default: false
Indicates whether it requires team manager's approval

userIds	
Array of strings unique
Default: "##default"
Represents set of user's identifier across the system

archived	
boolean
Default: false
Indicates whether the policy is archived.

automaticAccrual	
object (AutomaticAccrualDto)
Default: "##default"
Represents automatic approval settings.

amount	
number <double>
Represents automatic accrual's amount

period	
string
Default: "##default"
Enum: "MONTH" "YEAR"
Represents automatic accrual's period

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents automatic accrual's time unit

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
everyoneIncludingNew	
boolean
Default: false
Indicates whether the policy is applied to future new users.

id	
string
Default: "##default"
Represents policy identifier across the system.

name	
string
Default: "##default"
Represents the name of the policy.

negativeBalance	
object (NegativeBalanceDto)
Default: "##default"
Represents the data about negative balance including amount, time unit and period.

amount	
number <double>
period	
string
shouldReset	
boolean
timeUnit	
string
projectId	
string
Default: "##default"
Represents project identifier across the system.

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents the time unit of the policy.

userGroupIds	
Array of strings unique
Default: "##default"
Represents user groups' identifiers across the system. Indicates which user groups are included in the policy.

userIds	
Array of strings unique
Default: "##default"
Represents users' identifiers across the system. Indicates which users are included in the policy.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/time-off/policies
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-off/policies
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"allowHalfDay": false,
"allowNegativeBalance": true,
"approve": "##default",
"archived": true,
"automaticAccrual": "##default",
"automaticTimeEntryCreation": "##default",
"everyoneIncludingNew": false,
"id": "5b715612b079875110791111",
"name": "Days",
"negativeBalance": "##default",
"projectId": "##default",
"timeUnit": "DAYS",
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
Create a time off policy
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
allowHalfDay	
boolean
Default: false
Indicates whether policy allows half days.

allowNegativeBalance	
boolean
Default: false
Indicates whether policy allows negative balances.

approve
required
object (PolicyApprovalDto)
Default: "##default"
Represents approval settings.

requiresApproval	
boolean
Default: false
Indicates whether it requires approval

specificMembers	
boolean
Default: false
Indicates whether it requires specific members

teamManagers	
boolean
Default: false
Indicates whether it requires team manager's approval

userIds	
Array of strings unique
Default: "##default"
Represents set of user's identifier across the system

archived	
boolean
Default: false
Indicates whether policy is archived.

automaticAccrual	
object (AutomaticAccrualRequest)
Default: "##default"
Provide automatic accrual settings.

amount
required
number <double> >= 0
Represents amount of automatic accrual.

period	
string
Default: "##default"
Enum: "MONTH" "YEAR"
Represents automatic accrual period.

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents automatic accrual time unit.

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

everyoneIncludingNew	
boolean
Default: false
Indicates whether the policy is to be applied to future new users.

hasExpiration	
boolean
Default: false
Indicates whether the policy balance should have expiration

icon	
string
Default: "##default"
Enum: "UMBRELLA" "SNOWFLAKE" "FAMILY" "PLANE" "STETHOSCOPE" "HEALTH_METRICS" "CHILDCARE" "LUGGAGE" "MONETIZATION" "CALENDAR"
Provide icon.

name
required
string [ 2 .. 100 ] characters
Default: "##default"
Represents a name of new policy.

negativeBalance	
object (NegativeBalanceRequest)
Default: "##default"
Provide the negative balance data you would like to use for updating the policy.

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Indicates time unit of the policy.

userGroups	
object (UserGroupIdsSchema)
Default: "##default"
Provide list with user group ids and corresponding status.

users	
object (UserIdsSchema)
Default: "##default"
Provide list with user ids and corresponding status.

Responses
201 Created
Response Schema: application/json
allowHalfDay	
boolean
Default: false
Indicates whether the half day is allowed.

allowNegativeBalance	
boolean
Default: false
Indicates whether the negative balance is allowed.

approve	
object (PolicyApprovalDto)
Default: "##default"
Represents approval settings.

requiresApproval	
boolean
Default: false
Indicates whether it requires approval

specificMembers	
boolean
Default: false
Indicates whether it requires specific members

teamManagers	
boolean
Default: false
Indicates whether it requires team manager's approval

userIds	
Array of strings unique
Default: "##default"
Represents set of user's identifier across the system

archived	
boolean
Default: false
Indicates whether the policy is archived.

automaticAccrual	
object (AutomaticAccrualDto)
Default: "##default"
Represents automatic approval settings.

amount	
number <double>
Represents automatic accrual's amount

period	
string
Default: "##default"
Enum: "MONTH" "YEAR"
Represents automatic accrual's period

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents automatic accrual's time unit

automaticTimeEntryCreation	
object (AutomaticTimeEntryCreationDto)
Default: "##default"
Represents automatic time entry creation settings.

everyoneIncludingNew	
boolean
Default: false
Indicates whether the policy is applied to future new users.

id	
string
Default: "##default"
Represents policy identifier across the system.

name	
string
Default: "##default"
Represents the name of the policy.

negativeBalance	
object (NegativeBalanceDto)
Default: "##default"
Represents the data about negative balance including amount, time unit and period.

amount	
number <double>
period	
string
shouldReset	
boolean
timeUnit	
string
projectId	
string
Default: "##default"
Represents project identifier across the system.

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents the time unit of the policy.

userGroupIds	
Array of strings unique
Default: "##default"
Represents user groups' identifiers across the system. Indicates which user groups are included in the policy.

userIds	
Array of strings unique
Default: "##default"
Represents users' identifiers across the system. Indicates which users are included in the policy.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/time-off/policies
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-off/policies
Request samples
Payload
Content type
application/json

Copy
{
"allowHalfDay": false,
"allowNegativeBalance": true,
"approve": "##default",
"archived": true,
"automaticAccrual": "##default",
"automaticTimeEntryCreation": "##default",
"color": "#8BC34A",
"everyoneIncludingNew": false,
"hasExpiration": false,
"icon": "UMBRELLA",
"name": "Mental health days",
"negativeBalance": "##default",
"timeUnit": "DAYS",
"userGroups": "##default",
"users": "##default"
}
Response samples
201
Content type
application/json

Copy
Expand allCollapse all
{
"allowHalfDay": false,
"allowNegativeBalance": true,
"approve": "##default",
"archived": true,
"automaticAccrual": "##default",
"automaticTimeEntryCreation": "##default",
"everyoneIncludingNew": false,
"id": "5b715612b079875110791111",
"name": "Days",
"negativeBalance": "##default",
"projectId": "##default",
"timeUnit": "DAYS",
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
Delete a policy
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

id
required
string
Default: "##default"
Example: 63034cd0cb0fb876a57e93ad
Represents a policy identifier across the system.

Responses
200 OK

delete
/v1/workspaces/{workspaceId}/time-off/policies/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-off/policies/{id}
Get a time off policy
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

id
required
string
Default: "##default"
Example: 63034cd0cb0fb876a57e93ad
Represents a policy identifier across the system.

Responses
200 OK
Response Schema: application/json
allowHalfDay	
boolean
Default: false
Indicates whether the half day is allowed.

allowNegativeBalance	
boolean
Default: false
Indicates whether the negative balance is allowed.

approve	
object (PolicyApprovalDto)
Default: "##default"
Represents approval settings.

requiresApproval	
boolean
Default: false
Indicates whether it requires approval

specificMembers	
boolean
Default: false
Indicates whether it requires specific members

teamManagers	
boolean
Default: false
Indicates whether it requires team manager's approval

userIds	
Array of strings unique
Default: "##default"
Represents set of user's identifier across the system

archived	
boolean
Default: false
Indicates whether the policy is archived.

automaticAccrual	
object (AutomaticAccrualDto)
Default: "##default"
Represents automatic approval settings.

amount	
number <double>
Represents automatic accrual's amount

period	
string
Default: "##default"
Enum: "MONTH" "YEAR"
Represents automatic accrual's period

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents automatic accrual's time unit

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
everyoneIncludingNew	
boolean
Default: false
Indicates whether the policy is applied to future new users.

id	
string
Default: "##default"
Represents policy identifier across the system.

name	
string
Default: "##default"
Represents the name of the policy.

negativeBalance	
object (NegativeBalanceDto)
Default: "##default"
Represents the data about negative balance including amount, time unit and period.

amount	
number <double>
period	
string
shouldReset	
boolean
timeUnit	
string
projectId	
string
Default: "##default"
Represents project identifier across the system.

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents the time unit of the policy.

userGroupIds	
Array of strings unique
Default: "##default"
Represents user groups' identifiers across the system. Indicates which user groups are included in the policy.

userIds	
Array of strings unique
Default: "##default"
Represents users' identifiers across the system. Indicates which users are included in the policy.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/time-off/policies/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-off/policies/{id}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"allowHalfDay": false,
"allowNegativeBalance": true,
"approve": "##default",
"archived": true,
"automaticAccrual": "##default",
"automaticTimeEntryCreation": "##default",
"everyoneIncludingNew": false,
"id": "5b715612b079875110791111",
"name": "Days",
"negativeBalance": "##default",
"projectId": "##default",
"timeUnit": "DAYS",
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
Change a policy status
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

id
required
string
Default: "##default"
Example: 63034cd0cb0fb876a57e93ad
Represents a policy identifier across the system.

Request Body schema: application/json
required
status
required
string
Default: "##default"
Enum: "ACTIVE" "ARCHIVED" "ALL"
Provide the status you would like to use for changing the policy.

Responses
200 OK
Response Schema: application/json
allowHalfDay	
boolean
Default: false
Indicates whether the half day is allowed.

allowNegativeBalance	
boolean
Default: false
Indicates whether the negative balance is allowed.

approve	
object (PolicyApprovalDto)
Default: "##default"
Represents approval settings.

requiresApproval	
boolean
Default: false
Indicates whether it requires approval

specificMembers	
boolean
Default: false
Indicates whether it requires specific members

teamManagers	
boolean
Default: false
Indicates whether it requires team manager's approval

userIds	
Array of strings unique
Default: "##default"
Represents set of user's identifier across the system

archived	
boolean
Default: false
Indicates whether the policy is archived.

automaticAccrual	
object (AutomaticAccrualDto)
Default: "##default"
Represents automatic approval settings.

amount	
number <double>
Represents automatic accrual's amount

period	
string
Default: "##default"
Enum: "MONTH" "YEAR"
Represents automatic accrual's period

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents automatic accrual's time unit

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
everyoneIncludingNew	
boolean
Default: false
Indicates whether the policy is applied to future new users.

id	
string
Default: "##default"
Represents policy identifier across the system.

name	
string
Default: "##default"
Represents the name of the policy.

negativeBalance	
object (NegativeBalanceDto)
Default: "##default"
Represents the data about negative balance including amount, time unit and period.

amount	
number <double>
period	
string
shouldReset	
boolean
timeUnit	
string
projectId	
string
Default: "##default"
Represents project identifier across the system.

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents the time unit of the policy.

userGroupIds	
Array of strings unique
Default: "##default"
Represents user groups' identifiers across the system. Indicates which user groups are included in the policy.

userIds	
Array of strings unique
Default: "##default"
Represents users' identifiers across the system. Indicates which users are included in the policy.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


patch
/v1/workspaces/{workspaceId}/time-off/policies/{id}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-off/policies/{id}
Request samples
Payload
Content type
application/json

Copy
{
"status": "ACTIVE"
}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
{
"allowHalfDay": false,
"allowNegativeBalance": true,
"approve": "##default",
"archived": true,
"automaticAccrual": "##default",
"automaticTimeEntryCreation": "##default",
"everyoneIncludingNew": false,
"id": "5b715612b079875110791111",
"name": "Days",
"negativeBalance": "##default",
"projectId": "##default",
"timeUnit": "DAYS",
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
Update a policy
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

id
required
string
Default: "##default"
Example: 63034cd0cb0fb876a57e93ad
Represents a policy identifier across the system.

Request Body schema: application/json
required
allowHalfDay
required
boolean
Default: false
Indicates whether policy allows half day.

allowNegativeBalance
required
boolean
Default: false
Indicates whether policy allows negative balance.

approve
required
object (PolicyApprovalDto)
Default: "##default"
Represents approval settings.

requiresApproval	
boolean
Default: false
Indicates whether it requires approval

specificMembers	
boolean
Default: false
Indicates whether it requires specific members

teamManagers	
boolean
Default: false
Indicates whether it requires team manager's approval

userIds	
Array of strings unique
Default: "##default"
Represents set of user's identifier across the system

archived
required
boolean
Default: false
Indicates whether policy is archived.

automaticAccrual	
object (AutomaticAccrualRequest)
Default: "##default"
Provide automatic accrual settings.

amount
required
number <double> >= 0
Represents amount of automatic accrual.

period	
string
Default: "##default"
Enum: "MONTH" "YEAR"
Represents automatic accrual period.

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents automatic accrual time unit.

automaticTimeEntryCreation	
object (AutomaticTimeEntryCreationRequest)
Default: "##default"
Provides automatic time entry creation settings.

color	
string^#(?:[0-9a-fA-F]{6}){1}$
Default: "##default"
Provide color in format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

everyoneIncludingNew
required
boolean
Default: false
Indicates whether the policy is shown to new users.

hasExpiration
required
boolean
Default: false
Indicates whether the policy has expiration.

icon	
string
Default: "##default"
Enum: "UMBRELLA" "SNOWFLAKE" "FAMILY" "PLANE" "STETHOSCOPE" "HEALTH_METRICS" "CHILDCARE" "LUGGAGE" "MONETIZATION" "CALENDAR"
Provide icon.

name
required
string [ 2 .. 100 ] characters
Default: "##default"
Provide the name you would like to use for updating the policy.

negativeBalance	
object (NegativeBalanceRequest)
Default: "##default"
Provide the negative balance data you would like to use for updating the policy.

amount
required
number <double> >= 0
Represents negative balance amount.

amountValidForTimeUnit	
boolean
period	
string
Default: "##default"
Enum: "MONTH" "YEAR"
Represents negative balance period.

shouldReset	
boolean
Default: false
Indicates whether negative balance should be reset at the end of the negative balance period.

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents negative balance time unit.

userGroups
required
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
required
object (UserIdsSchema)
Default: "##default"
Provide list with user ids and corresponding status.

Responses
200 OK
Response Schema: application/json
allowHalfDay	
boolean
Default: false
Indicates whether the half day is allowed.

allowNegativeBalance	
boolean
Default: false
Indicates whether the negative balance is allowed.

approve	
object (PolicyApprovalDto)
Default: "##default"
Represents approval settings.

requiresApproval	
boolean
Default: false
Indicates whether it requires approval

specificMembers	
boolean
Default: false
Indicates whether it requires specific members

teamManagers	
boolean
Default: false
Indicates whether it requires team manager's approval

userIds	
Array of strings unique
Default: "##default"
Represents set of user's identifier across the system

archived	
boolean
Default: false
Indicates whether the policy is archived.

automaticAccrual	
object (AutomaticAccrualDto)
Default: "##default"
Represents automatic approval settings.

amount	
number <double>
Represents automatic accrual's amount

period	
string
Default: "##default"
Enum: "MONTH" "YEAR"
Represents automatic accrual's period

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents automatic accrual's time unit

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
everyoneIncludingNew	
boolean
Default: false
Indicates whether the policy is applied to future new users.

id	
string
Default: "##default"
Represents policy identifier across the system.

name	
string
Default: "##default"
Represents the name of the policy.

negativeBalance	
object (NegativeBalanceDto)
Default: "##default"
Represents the data about negative balance including amount, time unit and period.

projectId	
string
Default: "##default"
Represents project identifier across the system.

timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents the time unit of the policy.

userGroupIds	
Array of strings unique
Default: "##default"
Represents user groups' identifiers across the system. Indicates which user groups are included in the policy.

userIds	
Array of strings unique
Default: "##default"
Represents users' identifiers across the system. Indicates which users are included in the policy.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.

