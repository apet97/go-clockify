Create a time off request
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

policyId
required
string
Default: "##default"
Example: 63034cd0cb0fb876a57e93ad
Represents a policy identifier across the system.

Request Body schema: application/json
required
note	
string
Default: "##default"
Provide the note you would like to use for creating the time off request.

timeOffPeriod
required
object (TimeOffRequestPeriodV1Request)
Default: "##default"
Provide the period you would like to use for creating the time off request. If timeZone isn't set, should be aligned with time zone for user in settings. Can be shifted from user time zone with explicit setting of timeZone.

halfDayPeriod	
string
Default: "##default"
Enum: "FIRST_HALF" "SECOND_HALF" "NOT_DEFINED"
Represents the half day period.

isHalfDay	
boolean
Default: false
Indicates whether time off is half day.

period
required
object (PeriodV1Request)
Default: "##default"
Represents period of time off request including start and end date.

days	
integer <int32> [ 1 .. 999 ]
Provide number of days.

end	
string
Default: "##default"
Provide end date in YYYY-MM-DD format.

start	
string
Default: "##default"
Provide start date in YYYY-MM-DD format.

timeOffHalfDayPeriod	
string
Enum: "FIRST_HALF" "SECOND_HALF" "NOT_DEFINED"
Responses
200 OK
Response Schema: application/json
balance	
number <double>
Represents the time off balance.

balanceDiff	
number <double>
Represents the balance difference.

createdAt	
string <date-time>
Represents the date when time off request is created. It is in format YYYY-MM-DDTHH:MM:SS.ssssssZ

id	
string
Default: "##default"
Represents time off requester identifier across the system.

note	
string
Default: "##default"
Represents the note of the time off request.

policyId	
string
Default: "##default"
Represents policy identifier across the system.

policyName	
string
Default: "##default"
Represents the policy name of the time off request.

requesterUserId	
string
Default: "##default"
Represents requester user's id.

requesterUserName	
string
Default: "##default"
Represents requester user's username.

status	
object (TimeOffRequestStatus)
Default: "##default"
Represents the status the time off request.

changedAt	
string <date-time>
changedByUserId	
string
changedByUserName	
string
changedForUserName	
string
note	
string
statusType	
string
Enum: "PENDING" "APPROVED" "REJECTED" "ALL"
timeOffPeriod	
object (TimeOffRequestPeriodDto)
Default: "##default"
Represents the period the time off request.

halfDay	
boolean
halfDayHours	
object (Period)
end	
string <date-time>
start	
string <date-time>
halfDayPeriod	
string
period	
object (Period)
end	
string <date-time>
start	
string <date-time>
timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents the time unit of the time off request.

userEmail	
string
Default: "##default"
Represents user's email

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user's username.

userTimeZone	
string
Default: "##default"
Represents user's time zone

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/time-off/policies/{policyId}/requests
Request samples
Payload
Content type
application/json

Copy
{
"note": "Create Time Off Note",
"timeOffPeriod": "##default"
}
Response samples
200
Content type
application/json

Copy
"##default"
Delete a time off request
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

policyId
required
string
Default: "##default"
Example: 63034cd0cb0fb876a57e93ad
Represents a policy identifier across the system.

requestId
required
string
Default: "##default"
Example: 6308850156b7d75ea8fd3fbd
Represents a time off request identifier across the system.

Responses
200 OK
Response Schema: application/json
balanceDiff	
number <double>
Represents the balance difference

createdAt	
string <date-time>
Represents the date when time off request is created. Date is in format YYYY-MM-DDTHH:MM:SS.ssssssZ

id	
string
Default: "##default"
Represents time off requester identifier across the system.

note	
string
Default: "##default"
Represents the note of the time off request.

policyId	
string
Default: "##default"
Represents policy identifier across the system.

status	
object (TimeOffRequestStatus)
Default: "##default"
Represents the status the time off request.

changedAt	
string <date-time>
changedByUserId	
string
changedByUserName	
string
changedForUserName	
string
note	
string
statusType	
string
Enum: "PENDING" "APPROVED" "REJECTED" "ALL"
timeOffPeriod	
object (TimeOffRequestPeriodDto)
Default: "##default"
Represents the period the time off request.

halfDay	
boolean
halfDayHours	
object (Period)
end	
string <date-time>
start	
string <date-time>
halfDayPeriod	
string
period	
object (Period)
userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


delete
/v1/workspaces/{workspaceId}/time-off/policies/{policyId}/requests/{requestId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-off/policies/{policyId}/requests/{requestId}
Response samples
200
Content type
application/json

Copy
{
"balanceDiff": 1,
"createdAt": "2022-08-26T08:32:01.640708Z",
"id": "5b715612b079875110791111",
"note": "Time Off Request Note",
"policyId": "5b715612b079875110792333",
"status": "##default",
"timeOffPeriod": "##default",
"userId": "5b715612b079875110794444",
"workspaceId": "5b715612b079875110792222"
}
Change a time off request status
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

policyId
required
string
Default: "##default"
Example: 63034cd0cb0fb876a57e93ad
Represents a policy identifier across the system.

requestId
required
string
Default: "##default"
Example: 6308850156b7d75ea8fd3fbd
Represents a time off request identifier across the system.

Request Body schema: application/json
required
note	
string
Default: "##default"
Provide the note you would like to use for changing the time off request.

status	
string
Default: "##default"
Enum: "APPROVED" "REJECTED"
Provide the status you would like to use for changing the time off request.

Responses
200 OK
Response Schema: application/json
balanceDiff	
number <double>
Represents the balance difference

createdAt	
string <date-time>
Represents the date when time off request is created. Date is in format YYYY-MM-DDTHH:MM:SS.ssssssZ

id	
string
Default: "##default"
Represents time off requester identifier across the system.

note	
string
Default: "##default"
Represents the note of the time off request.

policyId	
string
Default: "##default"
Represents policy identifier across the system.

status	
object (TimeOffRequestStatus)
Default: "##default"
Represents the status the time off request.

timeOffPeriod	
object (TimeOffRequestPeriodDto)
Default: "##default"
Represents the period the time off request.

halfDay	
boolean
halfDayHours	
object (Period)
end	
string <date-time>
start	
string <date-time>
halfDayPeriod	
string
period	
object (Period)
end	
string <date-time>
start	
string <date-time>
userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


patch
/v1/workspaces/{workspaceId}/time-off/policies/{policyId}/requests/{requestId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-off/policies/{policyId}/requests/{requestId}
Request samples
Payload
Content type
application/json

Copy
{
"note": "Time Off Request Note",
"status": "APPROVED"
}
Response samples
200
Content type
application/json

Copy
{
"balanceDiff": 1,
"createdAt": "2022-08-26T08:32:01.640708Z",
"id": "5b715612b079875110791111",
"note": "Time Off Request Note",
"policyId": "5b715612b079875110792333",
"status": "##default",
"timeOffPeriod": "##default",
"userId": "5b715612b079875110794444",
"workspaceId": "5b715612b079875110792222"
}
Create a time off request for a user
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 60f91b3ffdaf031696ec61a8
Represents a workspace identifier across the system.

policyId
required
string
Default: "##default"
Example: 63034cd0cb0fb876a57e93ad
Represents a policy identifier across the system.

userId
required
string
Default: "##default"
Example: 60f924bafdaf031696ec6218
Represents a user identifier across the system.

Request Body schema: application/json
required
note	
string
Default: "##default"
Provide the note you would like to use for creating the time off request.

timeOffPeriod
required
object (TimeOffRequestPeriodV1Request)
Default: "##default"
Provide the period you would like to use for creating the time off request. If timeZone isn't set, should be aligned with time zone for user in settings. Can be shifted from user time zone with explicit setting of timeZone.

halfDayPeriod	
string
Default: "##default"
Enum: "FIRST_HALF" "SECOND_HALF" "NOT_DEFINED"
Represents the half day period.

isHalfDay	
boolean
Default: false
Indicates whether time off is half day.

period
required
object (PeriodV1Request)
Default: "##default"
Represents period of time off request including start and end date.

timeOffHalfDayPeriod	
string
Enum: "FIRST_HALF" "SECOND_HALF" "NOT_DEFINED"
Responses
200 OK
Response Schema: application/json
balance	
number <double>
Represents the time off balance.

balanceDiff	
number <double>
Represents the balance difference.

createdAt	
string <date-time>
Represents the date when time off request is created. It is in format YYYY-MM-DDTHH:MM:SS.ssssssZ

id	
string
Default: "##default"
Represents time off requester identifier across the system.

note	
string
Default: "##default"
Represents the note of the time off request.

policyId	
string
Default: "##default"
Represents policy identifier across the system.

policyName	
string
Default: "##default"
Represents the policy name of the time off request.

requesterUserId	
string
Default: "##default"
Represents requester user's id.

requesterUserName	
string
Default: "##default"
Represents requester user's username.

status	
object (TimeOffRequestStatus)
Default: "##default"
Represents the status the time off request.

changedAt	
string <date-time>
changedByUserId	
string
changedByUserName	
string
changedForUserName	
string
note	
string
statusType	
string
Enum: "PENDING" "APPROVED" "REJECTED" "ALL"
timeOffPeriod	
object (TimeOffRequestPeriodDto)
Default: "##default"
Represents the period the time off request.

halfDay	
boolean
halfDayHours	
object (Period)
end	
string <date-time>
start	
string <date-time>
halfDayPeriod	
string
period	
object (Period)
end	
string <date-time>
start	
string <date-time>
timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents the time unit of the time off request.

userEmail	
string
Default: "##default"
Represents user's email

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user's username.

userTimeZone	
string
Default: "##default"
Represents user's time zone

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/time-off/policies/{policyId}/users/{userId}/requests
https://api.clockify.me/api/v1/workspaces/{workspaceId}/time-off/policies/{policyId}/users/{userId}/requests
Request samples
Payload
Content type
application/json

Copy
{
"note": "Create Time Off Note",
"timeOffPeriod": "##default"
}
Response samples
200
Content type
application/json

Copy
"##default"
Get all time off requests on a workspace
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
end	
string <date-time>
Return time off requests created before the specified time in requester's time zone. Provide end in format YYYY-MM-DDTHH:MM:SS.ssssssZ

page	
integer <int32> <= 1000
Default: 1
Page number.

pageSize	
integer <int32> [ 1 .. 200 ]
Default: 50
Page size.

start	
string <date-time>
Return time off requests created after the specified time in requester's time zone. Provide start in format YYYY-MM-DDTHH:MM:SS.ssssssZ

statuses	
Array of strings unique
Default: "##default"
Items Enum: "PENDING" "APPROVED" "REJECTED" "ALL"
Filters time off requests by status.

userGroups	
Array of strings unique
Default: "##default"
Provide the user group ids of time off requests.

users	
Array of strings unique
Default: "##default"
Provide the user ids of time off requests. If empty, will return time off requests of all users (with a maximum of 5000 users).

Responses
200 OK
Response Schema: application/json
count	
integer <int32>
Total count of time off requests.

requests	
Array of objects (TimeOffRequestFullV1Dto)
Default: "##default"
Array 
balance	
number <double>
Represents the time off balance.

balanceDiff	
number <double>
Represents the balance difference.

createdAt	
string <date-time>
Represents the date when time off request is created. It is in format YYYY-MM-DDTHH:MM:SS.ssssssZ

id	
string
Default: "##default"
Represents time off requester identifier across the system.

note	
string
Default: "##default"
Represents the note of the time off request.

policyId	
string
Default: "##default"
Represents policy identifier across the system.

policyName	
string
Default: "##default"
Represents the policy name of the time off request.

requesterUserId	
string
Default: "##default"
Represents requester user's id.

requesterUserName	
string
Default: "##default"
Represents requester user's username.

status	
object (TimeOffRequestStatus)
Default: "##default"
Represents the status the time off request.

changedAt	
string <date-time>
changedByUserId	
string
changedByUserName	
string
changedForUserName	
string
note	
string
statusType	
string
Enum: "PENDING" "APPROVED" "REJECTED" "ALL"
timeOffPeriod	
object (TimeOffRequestPeriodDto)
Default: "##default"
Represents the period the time off request.

halfDay	
boolean
halfDayHours	
object (Period)
end	
string <date-time>
start	
string <date-time>
halfDayPeriod	
string
period	
object (Period)
end	
string <date-time>
start	
string <date-time>
timeUnit	
string
Default: "##default"
Enum: "DAYS" "HOURS"
Represents the time unit of the time off request.

userEmail	
string
Default: "##default"
Represents user's email

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user's username.

userTimeZone	
string
Default: "##default"
Represents user's time zone

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/time-off/requests
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"end": "2022-08-26T23:55:06.281873Z",
"page": 1,
"pageSize": 50,
"start": "2022-08-26T08:00:06.281873Z",
"statuses": [
"APPROVED",
"PENDING"
],
"userGroups": [
"5b715612b079875110791342",
"5b715612b079875110791324",
"5b715612b079875110793142"
],
"users": [
"5b715612b079875110791432",
"b715612b079875110791234"
]
}
Response samples
200
Content type
application/json

Copy
{
"count": 1,
"requests": "##default"
}
