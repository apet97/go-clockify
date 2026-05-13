Get approval requests
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
status	
string
Enum: "PENDING" "APPROVED" "WITHDRAWN_APPROVAL"
Example: status=PENDING
Filters results based on the provided approval state.

sort-column	
string
Enum: "ID" "USER_ID" "START" "UPDATED_AT"
Example: sort-column=START
Represents the column name to be used as sorting criteria.

sort-order	
string
Enum: "ASCENDING" "DESCENDING"
Example: sort-order=ASCENDING
Represents the sorting order.

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

Responses
200 OK
Response Schema: application/json
Array 
approvalRequest	
object (ApprovalRequestDtoV1)
Default: "##default"
Represents a valid approval request data transfer object.

creator	
object (ApprovalRequestCreatorDtoV1)
Default: "##default"
Represents approval request creator object.

userEmail	
string
Default: "##default"
Represents user email.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

dateRange	
object (DateRangeDto)
Default: "##default"
Represents date range object.

end	
string <date-time>
start	
string <date-time>
id	
string
Default: "##default"
Represents approval request identifier across the workspace.

owner	
object (ApprovalRequestOwnerDtoV1)
Default: "##default"
Represents approval request owner object.

startOfWeek	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

timeZone	
string
Default: "##default"
Represents time zone.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

status	
object (ApprovalRequestStatusDtoV1)
Default: "##default"
Represents approval request status object.

note	
string
Default: "##default"
Represents an approval requesst note.

state	
string
Default: "##default"
Enum: "PENDING" "APPROVED" "WITHDRAWN_SUBMISSION" "WITHDRAWN_APPROVAL" "REJECTED"
Represents approval state enum.

updatedAt	
string <date-time>
Represents a date in yyyy-MM-ddThh:mm:ssZ format.

updatedBy	
string
Default: "##default"
Represents user identifier across the system.

updatedByUserName	
string
Default: "##default"
Represents user name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.

approvedTime	
string
Default: "##default"
Represents a time duration.

billableAmount	
number <double>
billableTime	
string
Default: "##default"
Represents a time duration.

breakTime	
string
Default: "##default"
Represents a time duration.

costAmount	
number <double>
Represents an amount.

entries	
Array of objects (TimeEntryInfoDto)
Default: "##default"
Represents a list of time entry info data transfer objects.

Array 
approvalRequestId	
string
Default: "##default"
Represents approval identifier across the system.

billable	
boolean
Default: false
Indicates whether time entry is billable or not.

costRate	
object (RateDto)
Default: "##default"
Represents hourly rate object.

customFieldValues	
Array of objects (CustomFieldValueDto)
Default: "##default"
Represents a list of custom field value objects.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

sourceType	
string
Default: "##default"
Enum: "WORKSPACE" "PROJECT" "TIMEENTRY"
Represents a custom field value source type.

timeEntryId	
string
Default: "##default"
Represents time entry identifier across the system.

value	
object
Default: "##default"
Represents custom field value.

description	
string
Default: "##default"
Represents a time entry description.

hourlyRate	
object (RateDto)
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
Indicates whether time entry is locked or not.

project	
object (ProjectInfoDto)
Default: "##default"
Represents a project info object.

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

id	
string
Default: "##default"
Represents project identifier across the system.

name	
string
Default: "##default"
Represents a project name.

tags	
Array of objects (TagDto)
Default: "##default"
Represents a list of tag objects.

Array 
archived	
boolean
Default: false
Indicates whether tag is archived or not.

id	
string
Default: "##default"
Represents tag identifier across the system.

name	
string
Default: "##default"
Represents tag name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.

task	
object (TaskInfoDto)
Default: "##default"
Represents a project info object.

id	
string
Default: "##default"
Represents task identifier across the system.

name	
string
Default: "##default"
Represents task name.

timeInterval	
object (TimeIntervalDto)
Default: "##default"
Represents a time interval object.

duration	
string
Default: "##default"
Represents a time duration.

end	
string <date-time>
offsetEnd	
integer <int32>
offsetStart	
integer <int32>
start	
string <date-time>
timeZone	
string
zonedEnd	
string <date-time>
zonedStart	
string <date-time>
type	
string
Default: "##default"
Enum: "REGULAR" "BREAK" "HOLIDAY" "TIME_OFF"
Represents a time entry type enum.

expenseTotal	
number <double>
Represents an amount.

expenses	
Array of objects (ExpenseHydratedDto)
Default: "##default"
Represents a list of expense hydrated data transfer objects.

pendingTime	
string
Default: "##default"
Represents a time duration.

trackedTime	
string
Default: "##default"
Represents a time duration.


get
/v1/workspaces/{workspaceId}/approval-requests
https://api.clockify.me/api/v1/workspaces/{workspaceId}/approval-requests
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"approvalRequest": "##default",
"approvedTime": "PT1H30M",
"billableAmount": 2500,
"billableTime": "PT1H30M",
"breakTime": "PT1H30M",
"costAmount": 5000,
"entries": "##default",
"expenseTotal": 7500,
"expenses": "##default",
"pendingTime": "PT1H30M",
"trackedTime": "PT1H30M"
}
]
Submit approval request
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
period	
string
Default: "##default"
Enum: "WEEKLY" "SEMI_MONTHLY" "MONTHLY"
Specifies the approval period. It has to match the workspace approval period setting.

periodStart
required
string non-empty
Default: "##default"
Specifies an approval period start date in yyyy-MM-ddThh:mm:ssZ format.

Responses
201 Created
Response Schema: application/json
creator	
object (ApprovalRequestCreatorDtoV1)
Default: "##default"
Represents approval request creator object.

userEmail	
string
Default: "##default"
Represents user email.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

dateRange	
object (DateRangeDto)
Default: "##default"
Represents date range object.

end	
string <date-time>
start	
string <date-time>
id	
string
Default: "##default"
Represents approval request identifier across the workspace.

owner	
object (ApprovalRequestOwnerDtoV1)
Default: "##default"
Represents approval request owner object.

startOfWeek	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

timeZone	
string
Default: "##default"
Represents time zone.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

status	
object (ApprovalRequestStatusDtoV1)
Default: "##default"
Represents approval request status object.

note	
string
Default: "##default"
Represents an approval requesst note.

state	
string
Default: "##default"
Enum: "PENDING" "APPROVED" "WITHDRAWN_SUBMISSION" "WITHDRAWN_APPROVAL" "REJECTED"
Represents approval state enum.

updatedAt	
string <date-time>
Represents a date in yyyy-MM-ddThh:mm:ssZ format.

updatedBy	
string
Default: "##default"
Represents user identifier across the system.

updatedByUserName	
string
Default: "##default"
Represents user name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/approval-requests
https://api.clockify.me/api/v1/workspaces/{workspaceId}/approval-requests
Request samples
Payload
Content type
application/json

Copy
{
"period": "MONTHLY",
"periodStart": "2020-01-01T00:00:00.000Z"
}
Response samples
201
Content type
application/json

Copy
"##default"
Submit non pending/approved entries/expenses for approval to an existing approval request
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
period	
string
Default: "##default"
Enum: "WEEKLY" "SEMI_MONTHLY" "MONTHLY"
Specifies the approval period. It has to match the workspace approval period setting.

periodStart
required
string non-empty
Default: "##default"
Specifies an approval period start date in yyyy-MM-ddThh:mm:ssZ format.

Responses
200 OK
Response Schema: */*
creator	
object (ApprovalRequestCreatorDtoV1)
Default: "##default"
Represents approval request creator object.

userEmail	
string
Default: "##default"
Represents user email.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

dateRange	
object (DateRangeDto)
Default: "##default"
Represents date range object.

end	
string <date-time>
start	
string <date-time>
id	
string
Default: "##default"
Represents approval request identifier across the workspace.

owner	
object (ApprovalRequestOwnerDtoV1)
Default: "##default"
Represents approval request owner object.

startOfWeek	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

timeZone	
string
Default: "##default"
Represents time zone.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

status	
object (ApprovalRequestStatusDtoV1)
Default: "##default"
Represents approval request status object.

note	
string
Default: "##default"
Represents an approval requesst note.

state	
string
Default: "##default"
Enum: "PENDING" "APPROVED" "WITHDRAWN_SUBMISSION" "WITHDRAWN_APPROVAL" "REJECTED"
Represents approval state enum.

updatedAt	
string <date-time>
Represents a date in yyyy-MM-ddThh:mm:ssZ format.

updatedBy	
string
Default: "##default"
Represents user identifier across the system.

updatedByUserName	
string
Default: "##default"
Represents user name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/approval-requests/resubmit-entries-for-approval
https://api.clockify.me/api/v1/workspaces/{workspaceId}/approval-requests/resubmit-entries-for-approval
Request samples
Payload
Content type
application/json

Copy
{
"period": "MONTHLY",
"periodStart": "2020-01-01T00:00:00.000Z"
}
Submit an approval request for a user
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
period	
string
Default: "##default"
Enum: "WEEKLY" "SEMI_MONTHLY" "MONTHLY"
Specifies the approval period. It has to match the workspace approval period setting.

periodStart
required
string non-empty
Default: "##default"
Specifies an approval period start date in yyyy-MM-ddThh:mm:ssZ format.

Responses
201 Created
Response Schema: application/json
creator	
object (ApprovalRequestCreatorDtoV1)
Default: "##default"
Represents approval request creator object.

userEmail	
string
Default: "##default"
Represents user email.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

dateRange	
object (DateRangeDto)
Default: "##default"
Represents date range object.

end	
string <date-time>
start	
string <date-time>
id	
string
Default: "##default"
Represents approval request identifier across the workspace.

owner	
object (ApprovalRequestOwnerDtoV1)
Default: "##default"
Represents approval request owner object.

startOfWeek	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

timeZone	
string
Default: "##default"
Represents time zone.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

status	
object (ApprovalRequestStatusDtoV1)
Default: "##default"
Represents approval request status object.

note	
string
Default: "##default"
Represents an approval requesst note.

state	
string
Default: "##default"
Enum: "PENDING" "APPROVED" "WITHDRAWN_SUBMISSION" "WITHDRAWN_APPROVAL" "REJECTED"
Represents approval state enum.

updatedAt	
string <date-time>
Represents a date in yyyy-MM-ddThh:mm:ssZ format.

updatedBy	
string
Default: "##default"
Represents user identifier across the system.

updatedByUserName	
string
Default: "##default"
Represents user name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/approval-requests/users/{userId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/approval-requests/users/{userId}
Request samples
Payload
Content type
application/json

Copy
{
"period": "MONTHLY",
"periodStart": "2020-01-01T00:00:00.000Z"
}
Response samples
201
Content type
application/json

Copy
"##default"
Re-submit rejected/withdrawn entries/expenses for an approval of a user
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
period	
string
Default: "##default"
Enum: "WEEKLY" "SEMI_MONTHLY" "MONTHLY"
Specifies the approval period. It has to match the workspace approval period setting.

periodStart
required
string non-empty
Default: "##default"
Specifies an approval period start date in yyyy-MM-ddThh:mm:ssZ format.

Responses
200 OK
Response Schema: */*
creator	
object (ApprovalRequestCreatorDtoV1)
Default: "##default"
Represents approval request creator object.

userEmail	
string
Default: "##default"
Represents user email.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

dateRange	
object (DateRangeDto)
Default: "##default"
Represents date range object.

end	
string <date-time>
start	
string <date-time>
id	
string
Default: "##default"
Represents approval request identifier across the workspace.

owner	
object (ApprovalRequestOwnerDtoV1)
Default: "##default"
Represents approval request owner object.

startOfWeek	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

timeZone	
string
Default: "##default"
Represents time zone.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

status	
object (ApprovalRequestStatusDtoV1)
Default: "##default"
Represents approval request status object.

note	
string
Default: "##default"
Represents an approval requesst note.

state	
string
Default: "##default"
Enum: "PENDING" "APPROVED" "WITHDRAWN_SUBMISSION" "WITHDRAWN_APPROVAL" "REJECTED"
Represents approval state enum.

updatedAt	
string <date-time>
Represents a date in yyyy-MM-ddThh:mm:ssZ format.

updatedBy	
string
Default: "##default"
Represents user identifier across the system.

updatedByUserName	
string
Default: "##default"
Represents user name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/approval-requests/users/{userId}/resubmit-entries-for-approval
https://api.clockify.me/api/v1/workspaces/{workspaceId}/approval-requests/users/{userId}/resubmit-entries-for-approval
Request samples
Payload
Content type
application/json

Copy
{
"period": "MONTHLY",
"periodStart": "2020-01-01T00:00:00.000Z"
}
Update an approval request
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

approvalRequestId
required
string
Default: "##default"
Example: 940ab5acb07987125438b65y
Represents an approval request identifier across the system.

Request Body schema: application/json
required
note	
string
Default: "##default"
Additional notes for the approval request.

state
required
string
Default: "##default"
Enum: "PENDING" "APPROVED" "WITHDRAWN_SUBMISSION" "WITHDRAWN_APPROVAL" "REJECTED"
Specifies the approval state to set.

Responses
200 OK
Response Schema: application/json
creator	
object (ApprovalRequestCreatorDtoV1)
Default: "##default"
Represents approval request creator object.

userEmail	
string
Default: "##default"
Represents user email.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

dateRange	
object (DateRangeDto)
Default: "##default"
Represents date range object.

end	
string <date-time>
start	
string <date-time>
id	
string
Default: "##default"
Represents approval request identifier across the workspace.

owner	
object (ApprovalRequestOwnerDtoV1)
Default: "##default"
Represents approval request owner object.

startOfWeek	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

timeZone	
string
Default: "##default"
Represents time zone.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

status	
object (ApprovalRequestStatusDtoV1)
Default: "##default"
Represents approval request status object.

note	
string
Default: "##default"
Represents an approval requesst note.

state	
string
Default: "##default"
Enum: "PENDING" "APPROVED" "WITHDRAWN_SUBMISSION" "WITHDRAWN_APPROVAL" "REJECTED"
Represents approval state enum.

updatedAt	
string <date-time>
Represents a date in yyyy-MM-ddThh:mm:ssZ format.

updatedBy	
string
Default: "##default"
Represents user identifier across the system.

updatedByUserName	
string
Default: "##default"
Represents user name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.