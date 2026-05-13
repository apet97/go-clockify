Get all assignments
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
Default: ""
Example: name=Bugfixing
If provided, assignments will be filtered by name

start
required
string
Default: "##default"
Example: start=2020-01-01T00:00:00Z
Represents a start date in the yyyy-MM-ddThh:mm:ssZ format.

end
required
string
Default: "##default"
Example: end=2021-01-01T00:00:00Z
Represents a start date in the yyyy-MM-ddThh:mm:ssZ format.

sort-column	
string
Enum: "PROJECT" "USER" "ID"
Example: sort-column=USER
Represents the column as the sorting criteria.

sort-order	
string
Enum: "ASCENDING" "DESCENDING"
Example: sort-order=ASCENDING
Represents the sorting mode.

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
billable	
boolean
Default: false
Indicates whether assignment is billable or not.

clientId	
string
Default: "##default"
Represents client identifier across the system.

clientName	
string
Default: "##default"
Represents project name.

hoursPerDay	
number <double>
Represents number of hours per day as double.

id	
string
Default: "##default"
Represents assignment identifier across the system.

note	
string
Default: "##default"
Represents assignment note.

period	
object (DateRangeDto)
Default: "##default"
Represents date range object.

end	
string <date-time>
start	
string <date-time>
projectArchived	
boolean
projectBillable	
boolean
projectColor	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

projectId	
string
Default: "##default"
Represents project identifier across the system.

projectName	
string
Default: "##default"
Represents project name.

startTime	
string
Default: "##default"
Represents start time in hh:mm:ss format.

taskId	
string
Default: "##default"
Represents task identifier across the system.

taskName	
string
Default: "##default"
Represents task name.

userId	
string
Default: "##default"
Represents user identifier across the system.

userName	
string
Default: "##default"
Represents user name.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/scheduling/assignments/all
https://api.clockify.me/api/v1/workspaces/{workspaceId}/scheduling/assignments/all
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"billable": false,
"clientId": "36b687e29ae1f428e7ebe109",
"clientName": "Software Development",
"hoursPerDay": 7.5,
"id": "74a687e29ae1f428e7ebe505",
"note": "This is a sample note for an assignment.",
"period": "##default",
"projectArchived": true,
"projectBillable": true,
"projectColor": "#000000",
"projectId": "56b687e29ae1f428e7ebe504",
"projectName": "Software Development",
"startTime": "10:00:00",
"taskId": "36b687e29ae1f428e7ebe109",
"taskName": "Bugfixing",
"userId": "72k687e29ae1f428e7ebe109",
"userName": "John Doe",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Get all scheduled assignments per project
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
end
required
string <date-time>
Represents an end date in the yyyy-MM-ddThh:mm:ssZ format.

page	
integer <int32>
Default: 1
Page number.

pageSize	
integer <int32> <= 200
Default: 50
Page size.

search	
string
Default: "##default"
Represents a term for searching projects and clients by name.

start
required
string <date-time>
Represents a start date in the yyyy-MM-ddThh:mm:ssZ format.

statusFilter	
string
Default: "##default"
Enum: "PUBLISHED" "UNPUBLISHED" "ALL"
Filters assignments by status.

Responses
200 OK
Response Schema: application/json
Array 
assignments	
Array of objects (AssignmentPerDayDto)
Default: "##default"
Represents a list of assignment per day objects.

Array 
date	
string <date-time>
hasAssignment	
boolean
clientName	
string
Default: "##default"
Represents project name.

milestones	
Array of objects (MilestoneDto)
Default: "##default"
Represents a list of milestone objects.

Array 
date	
string <date-time>
Represents a date in yyyy-MM-ddThh:mm:ssZ format.

id	
string
Default: "##default"
Represents milestone identifier across the system.

name	
string
Default: "##default"
Represents milestone name.

projectId	
string
Default: "##default"
Represents project identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.

projectArchived	
boolean
Default: false
Indicates whether project is archived or not.

projectBillable	
boolean
Default: false
Indicates whether project is billable or not.

projectColor	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

projectId	
string
Default: "##default"
Represents project identifier across the system.

projectName	
string
Default: "##default"
Represents project name.

taskId	
string
Default: "##default"
Represents task identifier across the system.

taskName	
string
Default: "##default"
Represents task name.

totalHours	
number <double>
Represents project total hours as double.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/scheduling/assignments/projects/totals
https://api.clockify.me/api/v1/workspaces/{workspaceId}/scheduling/assignments/projects/totals
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
"assignments": "##default",
"clientName": "Software Development",
"milestones": "##default",
"projectArchived": false,
"projectBillable": false,
"projectColor": "#000000",
"projectId": "56b687e29ae1f428e7ebe504",
"projectName": "Software Development",
"taskId": "36b687e29ae1f428e7ebe109",
"taskName": "Bugfixing",
"totalHours": 490.5,
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Get all scheduled assignments on project
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
Example: 56b687e29ae1f428e7ebe504
Represents a project identifier across the system.

query Parameters
start
required
string
Default: "##default"
Example: start=2020-01-01T00:00:00Z
Represents a start date in the yyyy-MM-ddThh:mm:ssZ format.

end
required
string
Default: "##default"
Example: end=2021-01-01T00:00:00Z
Represents an end date in the yyyy-MM-ddThh:mm:ssZ format.

Responses
200 OK
Response Schema: application/json
assignments	
Array of objects (AssignmentPerDayDto)
Default: "##default"
Represents a list of assignment per day objects.

Array 
date	
string <date-time>
hasAssignment	
boolean
clientName	
string
Default: "##default"
Represents project name.

milestones	
Array of objects (MilestoneDto)
Default: "##default"
Represents a list of milestone objects.

projectArchived	
boolean
Default: false
Indicates whether project is archived or not.

projectBillable	
boolean
Default: false
Indicates whether project is billable or not.

projectColor	
string
Default: "##default"
Color format ^#(?:[0-9a-fA-F]{6}){1}$. Explanation: A valid color code should start with '#' and consist of six hexadecimal characters, representing a color in hexadecimal format. Color value is in standard RGB hexadecimal format.

projectId	
string
Default: "##default"
Represents project identifier across the system.

projectName	
string
Default: "##default"
Represents project name.

taskId	
string
Default: "##default"
Represents task identifier across the system.

taskName	
string
Default: "##default"
Represents task name.

totalHours	
number <double>
Represents project total hours as double.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/scheduling/assignments/projects/totals/{projectId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/scheduling/assignments/projects/totals/{projectId}
Response samples
200
Content type
application/json

Copy
{
"assignments": "##default",
"clientName": "Software Development",
"milestones": "##default",
"projectArchived": false,
"projectBillable": false,
"projectColor": "#000000",
"projectId": "56b687e29ae1f428e7ebe504",
"projectName": "Software Development",
"taskId": "36b687e29ae1f428e7ebe109",
"taskName": "Bugfixing",
"totalHours": 490.5,
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Publish assignments
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
end
required
string
Default: "##default"
Represents end date in yyyy-MM-ddThh:mm:ssZ format.

notifyUsers	
boolean
Default: false
Indicates whether to notify users when assignment is published.

search	
string
Default: "##default"
Represents a search string.

start
required
string
Default: "##default"
Represents start date in yyyy-MM-ddThh:mm:ssZ format.

userFilter	
object (ContainsUsersFilterRequestV1)
Default: "##default"
Represents a user filter request object.

contains	
string
Default: "##default"
Enum: "CONTAINS" "DOES_NOT_CONTAIN" "CONTAINS_ONLY"
Filter type.

ids	
Array of strings unique
Default: "##default"
Represents a list of filter identifiers.

sourceType	
string
Default: "##default"
Value: "USER_GROUP"
Valid authorization source type.

status	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Filters entities by status.

statuses	
Array of strings
Default: "##default"
Items Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Valid array of membership statuses.

userGroupFilter	
object (ContainsUserGroupFilterRequestV1)
Default: "##default"
Represents a user group filter request object.

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

viewType	
string
Default: "##default"
Enum: "PROJECTS" "TEAM" "ALL"
Represents view type.

Responses
200 OK

put
/v1/workspaces/{workspaceId}/scheduling/assignments/publish
https://api.clockify.me/api/v1/workspaces/{workspaceId}/scheduling/assignments/publish
Request samples
Payload
Content type
application/json

Copy
"##default"
Create a recurring assignment
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
Indicates whether assignment is billable or not.

end
required
string
Default: "##default"
Represents an end date in the yyyy-MM-ddThh:mm:ssZ format.

hoursPerDay
required
number <double>
Represents assignment total hours per day.

includeNonWorkingDays	
boolean
Default: false
Indicates whether to include non-working days or not.

note	
string [ 0 .. 100 ] characters
Default: "##default"
Represents an assignment note.

projectId
required
string non-empty
Default: "##default"
Represents a project identifier across the system.

recurringAssignment	
object (RecurringAssignmentRequestV1)
Default: "##default"
repeat	
boolean
Default: false
Indicates whether assignment is recurring or not.

weeks
required
integer <int32> [ 1 .. 99 ]
Indicates number of weeks for assignment.

start
required
string
Default: "##default"
Represents a start date in the yyyy-MM-ddThh:mm:ssZ format.

startTime	
string
Default: "##default"
Represents a start time in the hh:mm:ss format.

taskId	
string
Default: "##default"
Represents a task identifier across the system.

userId
required
string non-empty
Default: "##default"
Represents a user identifier across the system.

Responses
201 Created
Response Schema: application/json
Array 
billable	
boolean
Default: false
Indicates whether assignment is billable or not.

excludeDays	
Array of objects (SchedulingExcludeDay) unique
Default: "##default"
Represents a list of excluded days objects

Array 
date	
string <date-time>
Represents a datetimr in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "WEEKEND" "HOLIDAY" "TIME_OFF"
Represents the scheduling exclude day enum.

hoursPerDay	
number <double>
Represents assignment total hours per day.

id	
string
Default: "##default"
Represents assignment identifier across the system.

includeNonWorkingDays	
boolean
Default: false
Indicates whether assignment should include non-working days or not.

note	
string
Default: "##default"
Represents assignment note.

period	
object (DateRangeDto)
Default: "##default"
Represents date range object.

end	
string <date-time>
start	
string <date-time>
projectId	
string
Default: "##default"
Represents project identifier across the system.

published	
boolean
Default: false
Indicates whether assignment is published or not.

recurring	
object (RecurringAssignmentDto)
Default: "##default"
Represents recurring assignment object.

repeat	
boolean
Default: false
Indicates whether assignment is recurring or not.

seriesId	
string
Default: "##default"
Represents series identifier.

weeks	
integer <int32>
Represents number of weeks for thhis assignment.

startTime	
string
Default: "##default"
Represents start time in hh:mm:ss format.

taskId	
string
Default: "##default"
Represents task identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/scheduling/assignments/recurring
https://api.clockify.me/api/v1/workspaces/{workspaceId}/scheduling/assignments/recurring
Request samples
Payload
Content type
application/json

Copy
"##default"
Response samples
201
Content type
application/json

Copy
Expand allCollapse all
[
{
"billable": false,
"excludeDays": "##default",
"hoursPerDay": 7.5,
"id": "74a687e29ae1f428e7ebe505",
"includeNonWorkingDays": false,
"note": "This is a sample note for an assignment.",
"period": "##default",
"projectId": "56b687e29ae1f428e7ebe504",
"published": false,
"recurring": "##default",
"startTime": "10:00:00",
"taskId": "36b687e29ae1f428e7ebe109",
"userId": "72k687e29ae1f428e7ebe109",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Delete a recurring assignment
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

assignmentId
required
string
Default: "##default"
Example: 5b641568b07987035750505e
Represents an assignment identifier across the system.

query Parameters
seriesUpdateOption	
string
Enum: "THIS_ONE" "THIS_AND_FOLLOWING" "ALL"
Example: seriesUpdateOption=ALL
Represents a series option.

Responses
200 OK
Response Schema: application/json
Array 
billable	
boolean
Default: false
Indicates whether assignment is billable or not.

excludeDays	
Array of objects (SchedulingExcludeDay) unique
Default: "##default"
Represents a list of excluded days objects

Array 
date	
string <date-time>
Represents a datetimr in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "WEEKEND" "HOLIDAY" "TIME_OFF"
Represents the scheduling exclude day enum.

hoursPerDay	
number <double>
Represents assignment total hours per day.

id	
string
Default: "##default"
Represents assignment identifier across the system.

includeNonWorkingDays	
boolean
Default: false
Indicates whether assignment should include non-working days or not.

note	
string
Default: "##default"
Represents assignment note.

period	
object (DateRangeDto)
Default: "##default"
Represents date range object.

end	
string <date-time>
start	
string <date-time>
projectId	
string
Default: "##default"
Represents project identifier across the system.

published	
boolean
Default: false
Indicates whether assignment is published or not.

recurring	
object (RecurringAssignmentDto)
Default: "##default"
Represents recurring assignment object.

repeat	
boolean
Default: false
Indicates whether assignment is recurring or not.

seriesId	
string
Default: "##default"
Represents series identifier.

weeks	
integer <int32>
Represents number of weeks for thhis assignment.

startTime	
string
Default: "##default"
Represents start time in hh:mm:ss format.

taskId	
string
Default: "##default"
Represents task identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


delete
/v1/workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"billable": false,
"excludeDays": "##default",
"hoursPerDay": 7.5,
"id": "74a687e29ae1f428e7ebe505",
"includeNonWorkingDays": false,
"note": "This is a sample note for an assignment.",
"period": "##default",
"projectId": "56b687e29ae1f428e7ebe504",
"published": false,
"recurring": "##default",
"startTime": "10:00:00",
"taskId": "36b687e29ae1f428e7ebe109",
"userId": "72k687e29ae1f428e7ebe109",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Update a recurring assignment
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

assignmentId
required
string
Default: "##default"
Example: 5b641568b07987035750505e
Represents an assignment identifier across the system.

Request Body schema: application/json
required
billable	
boolean
Default: false
Indicates whether assignment is billable or not.

end
required
string
Default: "##default"
Represents an end date in the yyyy-MM-ddThh:mm:ssZ format.

hoursPerDay	
number <double>
Represents assignment total hours per day.

includeNonWorkingDays	
boolean
Default: false
Indicates whether to include non-working days or not.

note	
string [ 0 .. 100 ] characters
Default: "##default"
Represents an assignment note.

seriesUpdateOption	
string
Default: "##default"
Enum: "THIS_ONE" "THIS_AND_FOLLOWING" "ALL"
Valid series option

start
required
string
Default: "##default"
Represents start date in yyyy-MM-ddThh:mm:ssZ format.

startTime	
string
Default: "##default"
Represents a start time in the hh:mm:ss format.

taskId	
string
Default: "##default"
Represents task identifier across the system.

Responses
200 OK
Response Schema: application/json
Array 
billable	
boolean
Default: false
Indicates whether assignment is billable or not.

excludeDays	
Array of objects (SchedulingExcludeDay) unique
Default: "##default"
Represents a list of excluded days objects

Array 
date	
string <date-time>
Represents a datetimr in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "WEEKEND" "HOLIDAY" "TIME_OFF"
Represents the scheduling exclude day enum.

hoursPerDay	
number <double>
Represents assignment total hours per day.

id	
string
Default: "##default"
Represents assignment identifier across the system.

includeNonWorkingDays	
boolean
Default: false
Indicates whether assignment should include non-working days or not.

note	
string
Default: "##default"
Represents assignment note.

period	
object (DateRangeDto)
Default: "##default"
Represents date range object.

end	
string <date-time>
start	
string <date-time>
projectId	
string
Default: "##default"
Represents project identifier across the system.

published	
boolean
Default: false
Indicates whether assignment is published or not.

recurring	
object (RecurringAssignmentDto)
Default: "##default"
Represents recurring assignment object.

repeat	
boolean
Default: false
Indicates whether assignment is recurring or not.

seriesId	
string
Default: "##default"
Represents series identifier.

weeks	
integer <int32>
Represents number of weeks for thhis assignment.

startTime	
string
Default: "##default"
Represents start time in hh:mm:ss format.

taskId	
string
Default: "##default"
Represents task identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


patch
/v1/workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId}
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
"excludeDays": "##default",
"hoursPerDay": 7.5,
"id": "74a687e29ae1f428e7ebe505",
"includeNonWorkingDays": false,
"note": "This is a sample note for an assignment.",
"period": "##default",
"projectId": "56b687e29ae1f428e7ebe504",
"published": false,
"recurring": "##default",
"startTime": "10:00:00",
"taskId": "36b687e29ae1f428e7ebe109",
"userId": "72k687e29ae1f428e7ebe109",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Change the recurring period
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

assignmentId
required
string
Default: "##default"
Example: 5b641568b07987035750505e
Represents an assignment identifier across the system.

Request Body schema: application/json
required
repeat	
boolean
Default: false
Indicates whether assignment is recurring or not.

weeks
required
integer <int32> [ 1 .. 99 ]
Indicates number of weeks for assignment.

Responses
200 OK
Response Schema: application/json
Array 
billable	
boolean
Default: false
Indicates whether assignment is billable or not.

excludeDays	
Array of objects (SchedulingExcludeDay) unique
Default: "##default"
Represents a list of excluded days objects

Array 
date	
string <date-time>
Represents a datetimr in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "WEEKEND" "HOLIDAY" "TIME_OFF"
Represents the scheduling exclude day enum.

hoursPerDay	
number <double>
Represents assignment total hours per day.

id	
string
Default: "##default"
Represents assignment identifier across the system.

includeNonWorkingDays	
boolean
Default: false
Indicates whether assignment should include non-working days or not.

note	
string
Default: "##default"
Represents assignment note.

period	
object (DateRangeDto)
Default: "##default"
Represents date range object.

end	
string <date-time>
start	
string <date-time>
projectId	
string
Default: "##default"
Represents project identifier across the system.

published	
boolean
Default: false
Indicates whether assignment is published or not.

recurring	
object (RecurringAssignmentDto)
Default: "##default"
Represents recurring assignment object.

repeat	
boolean
Default: false
Indicates whether assignment is recurring or not.

seriesId	
string
Default: "##default"
Represents series identifier.

weeks	
integer <int32>
Represents number of weeks for thhis assignment.

startTime	
string
Default: "##default"
Represents start time in hh:mm:ss format.

taskId	
string
Default: "##default"
Represents task identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/scheduling/assignments/series/{assignmentId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/scheduling/assignments/series/{assignmentId}
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
"excludeDays": "##default",
"hoursPerDay": 7.5,
"id": "74a687e29ae1f428e7ebe505",
"includeNonWorkingDays": false,
"note": "This is a sample note for an assignment.",
"period": "##default",
"projectId": "56b687e29ae1f428e7ebe504",
"published": false,
"recurring": "##default",
"startTime": "10:00:00",
"taskId": "36b687e29ae1f428e7ebe109",
"userId": "72k687e29ae1f428e7ebe109",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Get total of users' capacity on workspace
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
end
required
string <date-time>
Represents an end date in the yyyy-MM-ddThh:mm:ssZ format.

page	
integer <int32>
Default: 1
Page number.

pageSize	
integer <int32> <= 200
Default: 50
Page size.

search	
string
Default: "##default"
Represents the keyword for searching users by name or email.

start
required
string <date-time>
Represents a start date in the yyyy-MM-ddThh:mm:ssZ format.

statusFilter	
string
Default: "##default"
Enum: "PUBLISHED" "UNPUBLISHED" "ALL"
Filters assignments by status.

userFilter	
object (ContainsUsersFilterRequestV1)
Default: "##default"
Represents a user filter request object.

contains	
string
Default: "##default"
Enum: "CONTAINS" "DOES_NOT_CONTAIN" "CONTAINS_ONLY"
Filter type.

ids	
Array of strings unique
Default: "##default"
Represents a list of filter identifiers.

sourceType	
string
Default: "##default"
Value: "USER_GROUP"
Valid authorization source type.

status	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Filters entities by status.

statuses	
Array of strings
Default: "##default"
Items Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Valid array of membership statuses.

userGroupFilter	
object (ContainsUserGroupFilterRequestV1)
Default: "##default"
Represents a user group filter request object.

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

Responses
200 OK
Response Schema: application/json
Array 
capacityPerDay	
number <double>
Represents capacity per day in seconds. For a 7hr work day, value is 25200.

totalHoursPerDay	
Array of objects (TotalsPerDayDto)
Default: "##default"
Represents total hours per day object.

Array 
date	
string <date-time>
totalHours	
number <double>
userId	
string
Default: "##default"
Represents user identifier across the system.

userImage	
string
Default: "##default"
Represents url path to user image.

userName	
string
Default: "##default"
Represents user name.

userStatus	
string
Default: "##default"
Represents user status.

workingDays	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents list of days of the week.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/scheduling/assignments/user-filter/totals
https://api.clockify.me/api/v1/workspaces/{workspaceId}/scheduling/assignments/user-filter/totals
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
"capacityPerDay": 25200,
"totalHoursPerDay": "##default",
"userId": "72k687e29ae1f428e7ebe109",
"userImage": "##default",
"userName": "John Doe",
"userStatus": "ACTIVE",
"workingDays": "[\"MONDAY\",\"TUESDAY\",\"WEDNESDAY\",\"THURSDAY\",\"FRIDAY\"]",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
Get total capacity of a user
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

start
required
string
Default: "##default"
Example: start=2020-01-01T00:00:00Z
Represents a start date in the yyyy-MM-ddThh:mm:ssZ format.

end
required
string
Default: "##default"
Example: end=2021-01-01T00:00:00Z
Represents an end date in the yyyy-MM-ddThh:mm:ssZ format.

Responses
200 OK
Response Schema: application/json
capacityPerDay	
number <double>
Represents capacity per day in seconds. For a 7hr work day, value is 25200.

totalHoursPerDay	
Array of objects (TotalsPerDayDto)
Default: "##default"
Represents total hours per day object.

Array 
date	
string <date-time>
totalHours	
number <double>
userId	
string
Default: "##default"
Represents user identifier across the system.

userImage	
string
Default: "##default"
Represents url path to user image.

userName	
string
Default: "##default"
Represents user name.

userStatus	
string
Default: "##default"
Represents user status.

workingDays	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents list of days of the week.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/scheduling/assignments/users/{userId}/totals
https://api.clockify.me/api/v1/workspaces/{workspaceId}/scheduling/assignments/users/{userId}/totals
Response samples
200
Content type
application/json

Copy
{
"capacityPerDay": 25200,
"totalHoursPerDay": "##default",
"userId": "72k687e29ae1f428e7ebe109",
"userImage": "##default",
"userName": "John Doe",
"userStatus": "ACTIVE",
"workingDays": "[\"MONDAY\",\"TUESDAY\",\"WEDNESDAY\",\"THURSDAY\",\"FRIDAY\"]",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Copy a scheduled assignment
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

assignmentId
required
string
Default: "##default"
Example: 5b641568b07987035750505e
Represents an assignment identifier across the system.

Request Body schema: application/json
required
seriesUpdateOption	
string
Default: "##default"
Enum: "THIS_ONE" "THIS_AND_FOLLOWING" "ALL"
Represents a series update option.

userId
required
string
Default: "##default"
Represents a user identifier across the system.

Responses
200 OK
Response Schema: application/json
Array 
billable	
boolean
Default: false
Indicates whether assignment is billable or not.

excludeDays	
Array of objects (SchedulingExcludeDay) unique
Default: "##default"
Represents a list of excluded days objects

Array 
date	
string <date-time>
Represents a datetimr in yyyy-MM-ddThh:mm:ssZ format.

type	
string
Default: "##default"
Enum: "WEEKEND" "HOLIDAY" "TIME_OFF"
Represents the scheduling exclude day enum.

hoursPerDay	
number <double>
Represents assignment total hours per day.

id	
string
Default: "##default"
Represents assignment identifier across the system.

includeNonWorkingDays	
boolean
Default: false
Indicates whether assignment should include non-working days or not.

note	
string
Default: "##default"
Represents assignment note.

period	
object (DateRangeDto)
Default: "##default"
Represents date range object.

end	
string <date-time>
start	
string <date-time>
projectId	
string
Default: "##default"
Represents project identifier across the system.

published	
boolean
Default: false
Indicates whether assignment is published or not.

recurring	
object (RecurringAssignmentDto)
Default: "##default"
Represents recurring assignment object.

repeat	
boolean
Default: false
Indicates whether assignment is recurring or not.

seriesId	
string
Default: "##default"
Represents series identifier.

weeks	
integer <int32>
Represents number of weeks for thhis assignment.

startTime	
string
Default: "##default"
Represents start time in hh:mm:ss format.

taskId	
string
Default: "##default"
Represents task identifier across the system.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/scheduling/assignments/{assignmentId}/copy
https://api.clockify.me/api/v1/workspaces/{workspaceId}/scheduling/assignments/{assignmentId}/copy
Request samples
Payload
Content type
application/json

Copy
{
"seriesUpdateOption": "THIS_ONE",
"userId": "72k687e29ae1f428e7ebe109"
}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"billable": false,
"excludeDays": "##default",
"hoursPerDay": 7.5,
"id": "74a687e29ae1f428e7ebe505",
"includeNonWorkingDays": false,
"note": "This is a sample note for an assignment.",
"period": "##default",
"projectId": "56b687e29ae1f428e7ebe504",
"published": false,
"recurring": "##default",
"startTime": "10:00:00",
"taskId": "36b687e29ae1f428e7ebe109",
"userId": "72k687e29ae1f428e7ebe109",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]