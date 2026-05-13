Add a photo
Authorizations:
ApiKeyAuthAddonKeyAuth
Request Body schema: multipart/form-data
file
required
string <binary>
Image to be uploaded

Responses
200 OK
Response Schema: application/json
name	
string
Default: "##default"
File name of the uploaded image

url	
string
Default: "##default"
The URL of the uploaded image in the server


post
/v1/file/image
https://api.clockify.me/api/v1/file/image
Response samples
200
Content type
application/json

Copy
{
"name": "image-01234567.jpg",
"url": "https://clockify.com/image-01234567.jpg"
}
Get currently logged-in user's info
Authorizations:
MarketplaceKeyAuthApiKeyAuthAddonKeyAuth
query Parameters
include-memberships	
boolean
Default: false
Example: include-memberships=true
If set to true, memberships will be included.

Responses
200 OK
Response Schema: application/json
activeWorkspace	
string
Default: "##default"
Represents user's active workspace identifier across the system.

customFields	
Array of objects (UserCustomFieldValueDtoV1)
Default: "##default"
Represents a list of value objects for user’s custom fields.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

customFieldName	
string
Default: "##default"
Represents custom field name.

customFieldType	
object (CustomFieldType)
Default: "##default"
Represents custom field type.

One of object
CHECKBOX	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
DROPDOWN_MULTIPLE	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
DROPDOWN_SINGLE	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
LINK	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
NUMBER	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
TXT	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
userId	
string
Default: "##default"
Represents user identifier across the system.

value	
object
Default: "##default"
Represents custom field value.

defaultWorkspace	
string
Default: "##default"
Represents user default workspace identifier across the system.

email	
string
Default: "##default"
Represents email address of the user.

id	
string
Default: "##default"
Represents user identifier across the system.

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
Represents name of the user.

profilePicture	
string
Default: "##default"
Represents profile image path of the user.

settings	
object (UserSettingsDtoV1)
Default: "##default"
Represents user settings object.

alerts	
boolean
Default: false
approval	
boolean
Default: false
collapseAllProjectLists	
boolean
Default: false
dashboardPinToTop	
boolean
Default: false
dashboardSelection	
string
Default: "##default"
Enum: "ME" "TEAM"
dashboardViewType	
string
Default: "##default"
Enum: "PROJECT" "BILLABILITY"
dateFormat
required
string non-empty
Default: "##default"
Represents a date format.

groupSimilarEntriesDisabled	
boolean
Default: false
invoiceReminders	
boolean
Default: false
isCompactViewOn	
boolean
Default: false
lang	
string
Default: "##default"
longRunning	
boolean
Default: false
multiFactorEnabled	
boolean
Default: false
myStartOfDay	
string
Default: "##default"
onboarding	
boolean
Default: false
projectListCollapse	
integer <int32>
projectPickerTaskFilter	
boolean
Default: false
pto	
boolean
Default: false
reminders	
boolean
Default: false
scheduledReports	
boolean
Default: false
scheduling	
boolean
Default: false
sendNewsletter	
boolean
Default: false
showOnlyWorkingDays	
boolean
Default: false
summaryReportSettings	
object (SummaryReportSettingsDtoV1)
Default: "##default"
Represents a summary report settings object.

group
required
string non-empty
Default: "##default"
subgroup
required
string non-empty
Default: "##default"
theme	
string
Default: "##default"
Enum: "DARK" "DEFAULT"
timeFormat
required
string non-empty
Default: "##default"
Enum: "HOUR12" "HOUR24"
Represents a time format enum.

timeTrackingManual	
boolean
Default: false
timeZone
required
string non-empty
Default: "##default"
Represents a valid timezone ID

weekStart	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

weeklyUpdates	
boolean
Default: false
status	
object (AccountStatus)
Default: "##default"
Represents account status enum.


get
/v1/user
https://api.clockify.me/api/v1/user
Response samples
200
Content type
application/json

Copy
{
"activeWorkspace": "64a687e29ae1f428e7ebe303",
"customFields": "##default",
"defaultWorkspace": "64a687e29ae1f428e7ebe303",
"email": "johndoe@example.com",
"id": "5a0ab5acb07987125438b60f",
"memberships": "##default",
"name": "John Doe",
"profilePicture": "https://www.url.com/profile-picture1234567890.png",
"settings": "##default",
"status": "ACTIVE"
}
Get a member's profile
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

Responses
200 OK
Response Schema: application/json
email	
string
Default: "##default"
Represents email address of the user.

hasPassword	
boolean
Default: false
Indicates whether user has password or none.

hasPendingApprovalRequest	
boolean
Default: false
Indicates whether user has pending approval request.

imageUrl	
string
Default: "##default"
Represents an image url.

name	
string
Default: "##default"
Represents name of the user.

userCustomFieldValues	
Array of objects (UserCustomFieldValueFullDtoV1)
Default: "##default"
Represents a list of value objects for user’s custom fields.

Array 
customField	
object (CustomFieldDtoV1)
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

customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents user custom field name.

sourceType	
string
Default: "##default"
Enum: "WORKSPACE" "USER"
Represents user custom field source type.

type	
string
Default: "##default"
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
Represents custom field type.

userId	
string
Default: "##default"
Represents user identifier across the system.

value	
object
Default: "##default"
Represents user custom field value.

weekStart	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

workCapacity	
string
Default: "##default"
Represents work capacity as a time duration in the ISO-8601 format.

workingDays	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a list of days of the week.

workspaceNumber	
integer <int32>
Represents the number of workspace(s) the user is associated to.


get
/v1/workspaces/{workspaceId}/member-profile/{userId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/member-profile/{userId}
Response samples
200
Content type
application/json

Copy
{
"email": "johndoe@example.com",
"hasPassword": false,
"hasPendingApprovalRequest": false,
"imageUrl": "https://www.url.com/imageurl-1234567890.jpg",
"name": "John Doe",
"userCustomFieldValues": "##default",
"weekStart": "MONDAY",
"workCapacity": "PT7H",
"workingDays": "[\"MONDAY\",\"TUESDAY\",\"WEDNESDAY\",\"THURSDAY\",\"FRIDAY\"]",
"workspaceNumber": 3
}
Update a member's profile
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
imageUrl	
string
Default: "##default"
Represents an image url. A field that can only be updated for limited users.

name	
string [ 1 .. 100 ] characters
Deprecated
Default: "##default"
This body field is deprecated and can only be updated for limited users. Represents name of the user and can be changed on the CAKE.com Account profile page.

removeProfileImage	
boolean
Default: false
Indicates whether to remove profile image or not. A field that can only be updated for limited users.

userCustomFields	
Array of objects (UpsertUserCustomFieldRequest)
Default: "##default"
Represents a list of upsert user custom field objects.

Array 
customFieldId
required
string
Default: "##default"
Represents custom field identifier across the system.

value	
object
Default: "##default"
Represents custom field value.

weekStart	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

workCapacity	
string
Default: "##default"
Represents work capacity as a time duration in the ISO-8601 format. For example, for a 7hr work day, input should be PT7H.

workingDays	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a list of days of the week.

Responses
200 OK
Response Schema: application/json
email	
string
Default: "##default"
Represents email address of the user.

hasPassword	
boolean
Default: false
Indicates whether user has password or none.

hasPendingApprovalRequest	
boolean
Default: false
Indicates whether user has pending approval request.

imageUrl	
string
Default: "##default"
Represents an image url.

name	
string
Default: "##default"
Represents name of the user.

userCustomFieldValues	
Array of objects (UserCustomFieldValueFullDtoV1)
Default: "##default"
Represents a list of value objects for user’s custom fields.

Array 
customField	
object (CustomFieldDtoV1)
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

customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

name	
string
Default: "##default"
Represents user custom field name.

sourceType	
string
Default: "##default"
Enum: "WORKSPACE" "USER"
Represents user custom field source type.

type	
string
Default: "##default"
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
Represents custom field type.

userId	
string
Default: "##default"
Represents user identifier across the system.

value	
object
Default: "##default"
Represents user custom field value.

weekStart	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

workCapacity	
string
Default: "##default"
Represents work capacity as a time duration in the ISO-8601 format.

workingDays	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a list of days of the week.

workspaceNumber	
integer <int32>
Represents the number of workspace(s) the user is associated to.


patch
/v1/workspaces/{workspaceId}/member-profile/{userId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/member-profile/{userId}
Request samples
Payload
Content type
application/json

Copy
{
"imageUrl": "https://www.url.com/imageurl-1234567890.jpg",
"name": "John Doe",
"removeProfileImage": false,
"userCustomFields": "##default",
"weekStart": "MONDAY",
"workCapacity": "PT7H",
"workingDays": "[\"MONDAY\",\"TUESDAY\",\"WEDNESDAY\",\"THURSDAY\",\"FRIDAY\"]"
}
Response samples
200
Content type
application/json

Copy
{
"email": "johndoe@example.com",
"hasPassword": false,
"hasPendingApprovalRequest": false,
"imageUrl": "https://www.url.com/imageurl-1234567890.jpg",
"name": "John Doe",
"userCustomFieldValues": "##default",
"weekStart": "MONDAY",
"workCapacity": "PT7H",
"workingDays": "[\"MONDAY\",\"TUESDAY\",\"WEDNESDAY\",\"THURSDAY\",\"FRIDAY\"]",
"workspaceNumber": 3
}
Find all users on a workspace
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
email	
string
Default: "##default"
Example: email=mail@example.com
If provided, you'll get a filtered list of users that contain the provided string in their email address.

project-id	
string
Example: project-id=21a687e29ae1f428e7ebe606
If provided, you'll get a list of users that have access to the project.

status	
string
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
Example: status=ACTIVE
If provided, you'll get a filtered list of users with the corresponding status.

account-statuses	
string
Example: account-statuses=LIMITED
If provided, you'll get a filtered list of users with the corresponding account status filter. If not, this will only filter ACTIVE, PENDING_EMAIL_VERIFICATION, and NOT_REGISTERED Users.

name	
string
Default: "##default"
Example: name=John
If provided, you'll get a filtered list of users that contain the provided string in their name

sort-column	
string
Enum: "ID" "EMAIL" "NAME" "NAME_LOWERCASE" "ACCESS" "HOURLYRATE" "COSTRATE"
Example: sort-column=ID
Sorting column criteria. Default value: EMAIL

sort-order	
string
Enum: "ASCENDING" "DESCENDING"
Example: sort-order=ASCENDING
Sorting mode. Default value: ASCENDING

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

memberships	
string
Enum: "ALL" "NONE" "WORKSPACE" "PROJECT" "USERGROUP"
Example: memberships=WORKSPACE
If provided, you'll get all users along with workspaces, groups, or projects they have access to. Default value is NONE.

include-roles
required
string
Default: "false"
If you pass along includeRoles=true, you'll get each user's detailed manager role (including projects and members which they manage)

Responses
200 OK
Response Schema: application/json
Array 
activeWorkspace	
string
Default: "##default"
Represents user's active workspace identifier across the system.

customFields	
Array of objects (UserCustomFieldValueDtoV1)
Default: "##default"
Represents a list of value objects for user’s custom fields.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

customFieldName	
string
Default: "##default"
Represents custom field name.

customFieldType	
object (CustomFieldType)
Default: "##default"
Represents custom field type.

userId	
string
Default: "##default"
Represents user identifier across the system.

value	
object
Default: "##default"
Represents custom field value.

defaultWorkspace	
string
Default: "##default"
Represents user default workspace identifier across the system.

email	
string
Default: "##default"
Represents email address of the user.

id	
string
Default: "##default"
Represents user identifier across the system.

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
Represents name of the user.

profilePicture	
string
Default: "##default"
Represents profile image path of the user.

settings	
object (UserSettingsDtoV1)
Default: "##default"
Represents user settings object.

alerts	
boolean
Default: false
approval	
boolean
Default: false
collapseAllProjectLists	
boolean
Default: false
dashboardPinToTop	
boolean
Default: false
dashboardSelection	
string
Default: "##default"
Enum: "ME" "TEAM"
dashboardViewType	
string
Default: "##default"
Enum: "PROJECT" "BILLABILITY"
dateFormat
required
string non-empty
Default: "##default"
Represents a date format.

groupSimilarEntriesDisabled	
boolean
Default: false
invoiceReminders	
boolean
Default: false
isCompactViewOn	
boolean
Default: false
lang	
string
Default: "##default"
longRunning	
boolean
Default: false
multiFactorEnabled	
boolean
Default: false
myStartOfDay	
string
Default: "##default"
onboarding	
boolean
Default: false
projectListCollapse	
integer <int32>
projectPickerTaskFilter	
boolean
Default: false
pto	
boolean
Default: false
reminders	
boolean
Default: false
scheduledReports	
boolean
Default: false
scheduling	
boolean
Default: false
sendNewsletter	
boolean
Default: false
showOnlyWorkingDays	
boolean
Default: false
summaryReportSettings	
object (SummaryReportSettingsDtoV1)
Default: "##default"
Represents a summary report settings object.

group
required
string non-empty
Default: "##default"
subgroup
required
string non-empty
Default: "##default"
theme	
string
Default: "##default"
Enum: "DARK" "DEFAULT"
timeFormat
required
string non-empty
Default: "##default"
Enum: "HOUR12" "HOUR24"
Represents a time format enum.

timeTrackingManual	
boolean
Default: false
timeZone
required
string non-empty
Default: "##default"
Represents a valid timezone ID

weekStart	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

weeklyUpdates	
boolean
Default: false
status	
object (AccountStatus)
Default: "##default"
Represents account status enum.

One of object
ACTIVE	
string
Enum: "ACTIVE" "PENDING_EMAIL_VERIFICATION" "DELETED" "NOT_REGISTERED" "LIMITED" "LIMITED_DELETED"
DELETED	
string
Enum: "ACTIVE" "PENDING_EMAIL_VERIFICATION" "DELETED" "NOT_REGISTERED" "LIMITED" "LIMITED_DELETED"
LIMITED	
string
Enum: "ACTIVE" "PENDING_EMAIL_VERIFICATION" "DELETED" "NOT_REGISTERED" "LIMITED" "LIMITED_DELETED"
LIMITED_DELETED	
string
Enum: "ACTIVE" "PENDING_EMAIL_VERIFICATION" "DELETED" "NOT_REGISTERED" "LIMITED" "LIMITED_DELETED"
NOT_REGISTERED	
string
Enum: "ACTIVE" "PENDING_EMAIL_VERIFICATION" "DELETED" "NOT_REGISTERED" "LIMITED" "LIMITED_DELETED"
PENDING_EMAIL_VERIFICATION	
string
Enum: "ACTIVE" "PENDING_EMAIL_VERIFICATION" "DELETED" "NOT_REGISTERED" "LIMITED" "LIMITED_DELETED"
active	
boolean
limitedAccount	
boolean
notRegistered	
boolean

get
/v1/workspaces/{workspaceId}/users
https://api.clockify.me/api/v1/workspaces/{workspaceId}/users
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"activeWorkspace": "64a687e29ae1f428e7ebe303",
"customFields": "##default",
"defaultWorkspace": "64a687e29ae1f428e7ebe303",
"email": "johndoe@example.com",
"id": "5a0ab5acb07987125438b60f",
"memberships": "##default",
"name": "John Doe",
"profilePicture": "https://www.url.com/profile-picture1234567890.png",
"settings": "##default",
"status": "ACTIVE"
}
]
Filter workspace users
Authorizations:
MarketplaceKeyAuthApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Request Body schema: application/json
required
accountStatuses	
Array of strings unique
Default: "##default"
If provided, you'll get a filtered list of users with the corresponding account status filter. If not, this will only filter ACTIVE, PENDING_EMAIL_VERIFICATION, and NOT_REGISTERED Users.

email	
string
Default: "##default"
If provided, you'll get a filtered list of users that contain the provided string in their email address.

includeRoles	
boolean
Default: false
If you pass along includeRoles=true, you'll get each user's detailed manager role (including projects and members for whom they're managers)

memberships	
string
Default: "NONE"
Enum: "ALL" "NONE" "WORKSPACE" "PROJECT" "USERGROUP"
If provided, you'll get all users along with workspaces, groups, or projects they have access to.

name	
string
Default: "##default"
If provided, you'll get a filtered list of users that contain the provided string in their name.

page	
integer <int32>
Default: 1
Page number.

pageSize	
integer <int32> >= 1
Default: 50
Page size.

projectId	
string
Default: "##default"
If provided, you'll get a list of users that have access to the project.

roles	
Array of strings unique
Default: "##default"
Items Enum: "WORKSPACE_ADMIN" "OWNER" "TEAM_MANAGER" "PROJECT_MANAGER"
If provided, you'll get a filtered list of users that have any of the specified roles. Owners are counted as admins when filtering.

sortColumn	
string
Default: "##default"
Enum: "ID" "EMAIL" "NAME" "NAME_LOWERCASE" "ACCESS" "HOURLYRATE" "COSTRATE"
Sorting criteria

sortOrder	
string
Default: "##default"
Enum: "ASCENDING" "DESCENDING"
Sorting mode

status	
string
Default: "##default"
Enum: "PENDING" "ACTIVE" "DECLINED" "INACTIVE" "ALL"
If provided, you'll get a filtered list of users with the corresponding status.

userGroups	
Array of strings unique
Default: "##default"
If provided, you'll get a list of users that belong to the specified user group IDs.

Responses
200 OK
Response Schema: application/json
Array 
activeWorkspace	
string
Default: "##default"
Represents user's active workspace identifier across the system.

customFields	
Array of objects (UserCustomFieldValueDtoV1)
Default: "##default"
Represents a list of value objects for user’s custom fields.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

customFieldName	
string
Default: "##default"
Represents custom field name.

customFieldType	
object (CustomFieldType)
Default: "##default"
Represents custom field type.

One of object
CHECKBOX	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
DROPDOWN_MULTIPLE	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
DROPDOWN_SINGLE	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
LINK	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
NUMBER	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
TXT	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
userId	
string
Default: "##default"
Represents user identifier across the system.

value	
object
Default: "##default"
Represents custom field value.

defaultWorkspace	
string
Default: "##default"
Represents user default workspace identifier across the system.

email	
string
Default: "##default"
Represents email address of the user.

id	
string
Default: "##default"
Represents user identifier across the system.

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
Represents name of the user.

profilePicture	
string
Default: "##default"
Represents profile image path of the user.

settings	
object (UserSettingsDtoV1)
Default: "##default"
Represents user settings object.

alerts	
boolean
Default: false
approval	
boolean
Default: false
collapseAllProjectLists	
boolean
Default: false
dashboardPinToTop	
boolean
Default: false
dashboardSelection	
string
Default: "##default"
Enum: "ME" "TEAM"
dashboardViewType	
string
Default: "##default"
Enum: "PROJECT" "BILLABILITY"
dateFormat
required
string non-empty
Default: "##default"
Represents a date format.

groupSimilarEntriesDisabled	
boolean
Default: false
invoiceReminders	
boolean
Default: false
isCompactViewOn	
boolean
Default: false
lang	
string
Default: "##default"
longRunning	
boolean
Default: false
multiFactorEnabled	
boolean
Default: false
myStartOfDay	
string
Default: "##default"
onboarding	
boolean
Default: false
projectListCollapse	
integer <int32>
projectPickerTaskFilter	
boolean
Default: false
pto	
boolean
Default: false
reminders	
boolean
Default: false
scheduledReports	
boolean
Default: false
scheduling	
boolean
Default: false
sendNewsletter	
boolean
Default: false
showOnlyWorkingDays	
boolean
Default: false
summaryReportSettings	
object (SummaryReportSettingsDtoV1)
Default: "##default"
Represents a summary report settings object.

group
required
string non-empty
Default: "##default"
subgroup
required
string non-empty
Default: "##default"
theme	
string
Default: "##default"
Enum: "DARK" "DEFAULT"
timeFormat
required
string non-empty
Default: "##default"
Enum: "HOUR12" "HOUR24"
Represents a time format enum.

timeTrackingManual	
boolean
Default: false
timeZone
required
string non-empty
Default: "##default"
Represents a valid timezone ID

weekStart	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

weeklyUpdates	
boolean
Default: false
status	
object (AccountStatus)
Default: "##default"
Represents account status enum.


post
/v1/workspaces/{workspaceId}/users/info
https://api.clockify.me/api/v1/workspaces/{workspaceId}/users/info
Request samples
Payload
Content type
application/json

Copy
Expand allCollapse all
{
"accountStatuses": [
"LIMITED",
"ACTIVE"
],
"email": "mail@example.com",
"includeRoles": false,
"memberships": "NONE",
"name": "John",
"page": 1,
"pageSize": 50,
"projectId": "21a687e29ae1f428e7ebe606",
"roles": [
"WORKSPACE_ADMIN",
"OWNER"
],
"sortColumn": "ID",
"sortOrder": "ASCENDING",
"status": "ACTIVE",
"userGroups": [
"5a0ab5acb07987125438b60f",
"72wab5acb07987125438b564"
]
}
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"activeWorkspace": "64a687e29ae1f428e7ebe303",
"customFields": "##default",
"defaultWorkspace": "64a687e29ae1f428e7ebe303",
"email": "johndoe@example.com",
"id": "5a0ab5acb07987125438b60f",
"memberships": "##default",
"name": "John Doe",
"profilePicture": "https://www.url.com/profile-picture1234567890.png",
"settings": "##default",
"status": "ACTIVE"
}
]
Update a user's custom field
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

customFieldId
required
string
Default: "##default"
Example: 5e4117fe8c625f38930d57b7
Represents custom field identifier across the system.

Request Body schema: application/json
required
value	
object
Default: "##default"
Represents custom field value.

Responses
201 Created

put
/v1/workspaces/{workspaceId}/users/{userId}/custom-field/{customFieldId}/value
https://api.clockify.me/api/v1/workspaces/{workspaceId}/users/{userId}/custom-field/{customFieldId}/value
Request samples
Payload
Content type
application/json

Copy
{
"value": "20231211-12345"
}
Response samples
201
Content type
application/json

Copy
"##default"
Find user's team manager
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
sort-column	
string
Enum: "ID" "EMAIL" "NAME" "NAME_LOWERCASE" "ACCESS" "HOURLYRATE" "COSTRATE"
Example: sort-column=ID
Sorting column criteria

sort-order	
string
Enum: "ASCENDING" "DESCENDING"
Example: sort-order=ASCENDING
Sorting mode

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
activeWorkspace	
string
Default: "##default"
Represents user's active workspace identifier across the system.

customFields	
Array of objects (UserCustomFieldValueDtoV1)
Default: "##default"
Represents a list of value objects for user’s custom fields.

Array 
customFieldId	
string
Default: "##default"
Represents custom field identifier across the system.

customFieldName	
string
Default: "##default"
Represents custom field name.

customFieldType	
object (CustomFieldType)
Default: "##default"
Represents custom field type.

One of object
CHECKBOX	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
DROPDOWN_MULTIPLE	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
DROPDOWN_SINGLE	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
LINK	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
NUMBER	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
TXT	
string
Enum: "TXT" "NUMBER" "DROPDOWN_SINGLE" "DROPDOWN_MULTIPLE" "CHECKBOX" "LINK"
userId	
string
Default: "##default"
Represents user identifier across the system.

value	
object
Default: "##default"
Represents custom field value.

defaultWorkspace	
string
Default: "##default"
Represents user default workspace identifier across the system.

email	
string
Default: "##default"
Represents email address of the user.

id	
string
Default: "##default"
Represents user identifier across the system.

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
Represents name of the user.

profilePicture	
string
Default: "##default"
Represents profile image path of the user.

settings	
object (UserSettingsDtoV1)
Default: "##default"
Represents user settings object.

alerts	
boolean
Default: false
approval	
boolean
Default: false
collapseAllProjectLists	
boolean
Default: false
dashboardPinToTop	
boolean
Default: false
dashboardSelection	
string
Default: "##default"
Enum: "ME" "TEAM"
dashboardViewType	
string
Default: "##default"
Enum: "PROJECT" "BILLABILITY"
dateFormat
required
string non-empty
Default: "##default"
Represents a date format.

groupSimilarEntriesDisabled	
boolean
Default: false
invoiceReminders	
boolean
Default: false
isCompactViewOn	
boolean
Default: false
lang	
string
Default: "##default"
longRunning	
boolean
Default: false
multiFactorEnabled	
boolean
Default: false
myStartOfDay	
string
Default: "##default"
onboarding	
boolean
Default: false
projectListCollapse	
integer <int32>
projectPickerTaskFilter	
boolean
Default: false
pto	
boolean
Default: false
reminders	
boolean
Default: false
scheduledReports	
boolean
Default: false
scheduling	
boolean
Default: false
sendNewsletter	
boolean
Default: false
showOnlyWorkingDays	
boolean
Default: false
summaryReportSettings	
object (SummaryReportSettingsDtoV1)
Default: "##default"
Represents a summary report settings object.

group
required
string non-empty
Default: "##default"
subgroup
required
string non-empty
Default: "##default"
theme	
string
Default: "##default"
Enum: "DARK" "DEFAULT"
timeFormat
required
string non-empty
Default: "##default"
Enum: "HOUR12" "HOUR24"
Represents a time format enum.

timeTrackingManual	
boolean
Default: false
timeZone
required
string non-empty
Default: "##default"
Represents a valid timezone ID

weekStart	
string
Default: "##default"
Enum: "MONDAY" "TUESDAY" "WEDNESDAY" "THURSDAY" "FRIDAY" "SATURDAY" "SUNDAY"
Represents a day of the week.

weeklyUpdates	
boolean
Default: false
status	
object (AccountStatus)
Default: "##default"
Represents account status enum.


get
/v1/workspaces/{workspaceId}/users/{userId}/managers
https://api.clockify.me/api/v1/workspaces/{workspaceId}/users/{userId}/managers
Response samples
200
Content type
application/json

Copy
Expand allCollapse all
[
{
"activeWorkspace": "64a687e29ae1f428e7ebe303",
"customFields": "##default",
"defaultWorkspace": "64a687e29ae1f428e7ebe303",
"email": "johndoe@example.com",
"id": "5a0ab5acb07987125438b60f",
"memberships": "##default",
"name": "John Doe",
"profilePicture": "https://www.url.com/profile-picture1234567890.png",
"settings": "##default",
"status": "ACTIVE"
}
]
Remove user's manager role
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
entityId
required
string non-empty
Default: "##default"
Represents an entity identifier across the system.

role
required
string
Default: "##default"
Enum: "WORKSPACE_ADMIN" "TEAM_MANAGER" "PROJECT_MANAGER"
Represents a valid role.

sourceType	
string
Default: "##default"
Value: "USER_GROUP"
Optional field used to indicate that the target of the operation is a user group, in which case the value USER_GROUP should be used, alongside a valid user group ID for the entityId field. If omitted, a user ID should be used for the entityId field.

Responses
204 No Content

delete
/v1/workspaces/{workspaceId}/users/{userId}/roles
Request samples
Payload
Content type
application/json

Copy
"##default"
Give manager role to a user
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
entityId
required
string non-empty
Default: "##default"
Represents an entity identifier across the system.

role
required
string
Default: "##default"
Enum: "WORKSPACE_ADMIN" "TEAM_MANAGER" "PROJECT_MANAGER"
Represents a valid role.

sourceType	
string
Default: "##default"
Value: "USER_GROUP"
Optional field used to indicate that the target of the operation is a user group, in which case the value USER_GROUP should be used, alongside a valid user group ID for the entityId field. If omitted, a user ID should be used for the entityId field.

Responses
201 Created

post
/v1/workspaces/{workspaceId}/users/{userId}/roles
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
"role": "##default",
"userId": "5a0ab5acb07987125438b60f",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
]
