Expense
Get all expenses on a workspace
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

user-id	
string
Example: user-id=5a0ab5acb07987125438b60f
If provided, you'll get a filtered list of expenses which match the provided string in the user ID linked to the expense.

Responses
200 OK
Response Schema: application/json
dailyTotals	
Array of objects (ExpenseDailyTotalsDtoV1)
Default: "##default"
Represents a list of expense daily total data transfer objects.

Array 
date	
string
Default: "##default"
Represents a date in yyyy-MM-dd format.

dateAsInstant	
string <date-time>
total	
number <double>
Represents expense total.

expenses	
object (ExpensesWithCountDtoV1)
Default: "##default"
Represents an expense with count data transfer object.

count	
integer <int32>
Represent result count.

expenses	
Array of objects (ExpenseHydratedDtoV1)
Default: "##default"
Represent a list of hydrated expense objects.

weeklyTotals	
Array of objects (ExpenseWeeklyTotalsDtoV1)
Default: "##default"
Represents a list of expense weekly total data transfer objects.

Array 
date	
string
Default: "##default"
Represents a date in yyyy-MM-dd format.

total	
number <double>
Represents expense total.


get
/v1/workspaces/{workspaceId}/expenses
https://api.clockify.me/api/v1/workspaces/{workspaceId}/expenses
Response samples
200
Content type
application/json

Copy
{
"dailyTotals": "##default",
"expenses": "##default",
"weeklyTotals": "##default"
}
Create an expense
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Request Body schema: multipart/form-data
amount
required
number <double> <= 92233720368547760
Represents an expense amount as the double data type.

billable	
boolean
Default: false
Indicates whether expense is billable or not.

categoryId
required
string
Default: "##default"
Represents a category identifier across the system.

date
required
string <date-time>
Provides a valid yyyy-MM-ddThh:mm:ssZ format date.

file
required
string <binary>
notes	
string [ 0 .. 3000 ] characters
Default: "##default"
Represents notes for an expense.

projectId
required
string
Default: "##default"
Represents a project identifier across the system.

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
billable	
boolean
Default: false
Indicates whether expense is billable or not.

categoryId	
string
Default: "##default"
Represents category identifier across the system.

date	
string
Default: "##default"
Represents a date in yyyy-MM-dd format.

fileId	
string
Default: "##default"
Represents file identifier across the system.

id	
string
Default: "##default"
Represents expense identifier across the system.

locked	
boolean
notes	
string
Default: "##default"
Represents notes for an expense.

projectId	
string
Default: "##default"
Represents project identifier across the system.

quantity	
number <double>
Represents expense quantity as double data type.

taskId	
string
Default: "##default"
Represents task identifier across the system.

total	
number <double>
Represents expense total as double data type.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/expenses
https://api.clockify.me/api/v1/workspaces/{workspaceId}/expenses
Response samples
201
Content type
application/json

Copy
{
"billable": false,
"categoryId": "45y687e29ae1f428e7ebe890",
"date": "2020-01-01",
"fileId": "745687e29ae1f428e7ebe890",
"id": "64c777ddd3fcab07cfbb210c",
"locked": true,
"notes": "This is a sample note for this expense.",
"projectId": "25b687e29ae1f428e7ebe123",
"quantity": 0.1,
"taskId": "25b687e29ae1f428e7ebe123",
"total": 10500.5,
"userId": "89b687e29ae1f428e7ebe912",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Get all expense categories
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
sort-column	
string
Value: "NAME"
Example: sort-column=NAME
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

archived	
boolean
Default: false
Example: archived=true
Flag to filter results based on whether category is archived or not.

name	
string
Default: "##default"
Example: name=procurement
If provided, you'll get a filtered list of expense categories that matches the provided string in their name.

Responses
200 OK

get
/v1/workspaces/{workspaceId}/expenses/categories
Response samples
200
Content type
application/json

Copy
{
"categories": "##default",
"count": 20
}
Add an expense category
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
hasUnitPrice	
boolean
Default: false
Flag whether expense category has unit price or none.

name
required
string [ 0 .. 250 ] characters
Default: "##default"
Represents a valid expense category name.

priceInCents	
integer <int32>
Represents price in cents as integer.

unit	
string
Default: "##default"
Represents a valid expense category unit.

Responses
201 Created
Response Schema: application/json
archived	
boolean
Default: false
Flag that indicates whether the expense category is archived or not.

hasUnitPrice	
boolean
Default: false
Represents whether expense category has unit price or none.

id	
string
Default: "##default"
Represents expense category identifier across the system.

name	
string
Default: "##default"
Represents expense category name.

priceInCents	
integer <int32>
Represents price in cents as integer.

unit	
string
Default: "##default"
Represents expense category unit.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


post
/v1/workspaces/{workspaceId}/expenses/categories
Request samples
Payload
Content type
application/json

Copy
{
"hasUnitPrice": false,
"name": "Procurement",
"priceInCents": 1000,
"unit": "piece"
}
Response samples
201
Content type
application/json

Copy
"##default"
Delete an expense category
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

categoryId
required
string
Default: "##default"
Example: 89a687e29ae1f428e7ebe567
Represents a category identifier across the system.

Responses
204 No Content

delete
/v1/workspaces/{workspaceId}/expenses/categories/{categoryId}
Update an expense category
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

categoryId
required
string
Default: "##default"
Example: 89a687e29ae1f428e7ebe567
Represents a category identifier across the system.

Request Body schema: application/json
required
hasUnitPrice	
boolean
Default: false
Flag whether expense category has unit price or none.

name
required
string [ 0 .. 250 ] characters
Default: "##default"
Represents a valid expense category name.

priceInCents	
integer <int32>
Represents price in cents as integer.

unit	
string
Default: "##default"
Represents a valid expense category unit.

Responses
200 OK
Response Schema: application/json
archived	
boolean
Default: false
Flag that indicates whether the expense category is archived or not.

hasUnitPrice	
boolean
Default: false
Represents whether expense category has unit price or none.

id	
string
Default: "##default"
Represents expense category identifier across the system.

name	
string
Default: "##default"
Represents expense category name.

priceInCents	
integer <int32>
Represents price in cents as integer.

unit	
string
Default: "##default"
Represents expense category unit.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/expenses/categories/{categoryId}
https://api.clockify.me/api/v1/workspaces/{workspaceId}/expenses/categories/{categoryId}
Request samples
Payload
Content type
application/json

Copy
{
"hasUnitPrice": false,
"name": "Procurement",
"priceInCents": 1000,
"unit": "piece"
}
Response samples
200
Content type
application/json

Copy
"##default"
Archive an expense category
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

categoryId
required
string
Default: "##default"
Example: 89a687e29ae1f428e7ebe567
Represents a category identifier across the system.

Request Body schema: application/json
required
archived	
boolean
Default: false
Flag whether to archive the expense category or not.

Responses
200 OK

patch
/v1/workspaces/{workspaceId}/expenses/categories/{categoryId}/status
Request samples
Payload
Content type
application/json

Copy
{
"archived": false
}
Response samples
200
Content type
application/json

Copy
"##default"
Delete an expense
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

expenseId
required
string
Default: "##default"
Example: 64c777ddd3fcab07cfbb210c
Represents an expense identifier across the system.

Responses
200 OK

delete
/v1/workspaces/{workspaceId}/expenses/{expenseId}
Get an expense by ID
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

expenseId
required
string
Default: "##default"
Example: 64c777ddd3fcab07cfbb210c
Represents an expense identifier across the system.

Responses
200 OK
Response Schema: application/json
billable	
boolean
Default: false
Indicates whether expense is billable or not.

categoryId	
string
Default: "##default"
Represents category identifier across the system.

date	
string
Default: "##default"
Represents a date in yyyy-MM-dd format.

fileId	
string
Default: "##default"
Represents file identifier across the system.

id	
string
Default: "##default"
Represents expense identifier across the system.

locked	
boolean
notes	
string
Default: "##default"
Represents notes for an expense.

projectId	
string
Default: "##default"
Represents project identifier across the system.

quantity	
number <double>
Represents expense quantity as double data type.

taskId	
string
Default: "##default"
Represents task identifier across the system.

total	
number <double>
Represents expense total as double data type.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


get
/v1/workspaces/{workspaceId}/expenses/{expenseId}
Response samples
200
Content type
application/json

Copy
{
"billable": false,
"categoryId": "45y687e29ae1f428e7ebe890",
"date": "2020-01-01",
"fileId": "745687e29ae1f428e7ebe890",
"id": "64c777ddd3fcab07cfbb210c",
"locked": true,
"notes": "This is a sample note for this expense.",
"projectId": "25b687e29ae1f428e7ebe123",
"quantity": 0.1,
"taskId": "25b687e29ae1f428e7ebe123",
"total": 10500.5,
"userId": "89b687e29ae1f428e7ebe912",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Update an expense
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

expenseId
required
string
Default: "##default"
Example: 64c777ddd3fcab07cfbb210c
Represents an expense identifier across the system.

Request Body schema: multipart/form-data
amount
required
number <double> [ 0 .. 92233720368547760 ]
Represents an expense amount as the double data type.

billable	
boolean
Default: false
Indicates whether expense is billable or not.

categoryId
required
string
Default: "##default"
Represents a category identifier across the system.

changeFields
required
Array of strings
Default: "##default"
Items Enum: "USER" "DATE" "PROJECT" "TASK" "CATEGORY" "NOTES" "AMOUNT" "BILLABLE" "FILE"
Represents a list of expense change fields.

date
required
string <date-time>
Provides a valid yyyy-MM-ddThh:mm:ssZ format date.

file
required
string <binary>
notes	
string [ 0 .. 3000 ] characters
Default: "##default"
Represents notes for an expense.

projectId	
string
Default: "##default"
Represents a project identifier across the system.

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
200 OK
Response Schema: application/json
billable	
boolean
Default: false
Indicates whether expense is billable or not.

categoryId	
string
Default: "##default"
Represents category identifier across the system.

date	
string
Default: "##default"
Represents a date in yyyy-MM-dd format.

fileId	
string
Default: "##default"
Represents file identifier across the system.

id	
string
Default: "##default"
Represents expense identifier across the system.

locked	
boolean
notes	
string
Default: "##default"
Represents notes for an expense.

projectId	
string
Default: "##default"
Represents project identifier across the system.

quantity	
number <double>
Represents expense quantity as double data type.

taskId	
string
Default: "##default"
Represents task identifier across the system.

total	
number <double>
Represents expense total as double data type.

userId	
string
Default: "##default"
Represents user identifier across the system.

workspaceId	
string
Default: "##default"
Represents workspace identifier across the system.


put
/v1/workspaces/{workspaceId}/expenses/{expenseId}
Response samples
200
Content type
application/json

Copy
{
"billable": false,
"categoryId": "45y687e29ae1f428e7ebe890",
"date": "2020-01-01",
"fileId": "745687e29ae1f428e7ebe890",
"id": "64c777ddd3fcab07cfbb210c",
"locked": true,
"notes": "This is a sample note for this expense.",
"projectId": "25b687e29ae1f428e7ebe123",
"quantity": 0.1,
"taskId": "25b687e29ae1f428e7ebe123",
"total": 10500.5,
"userId": "89b687e29ae1f428e7ebe912",
"workspaceId": "64a687e29ae1f428e7ebe303"
}
Download a receipt
Authorizations:
ApiKeyAuthAddonKeyAuth
path Parameters
fileId
required
string
Default: "##default"
Example: 745687e29ae1f428e7ebe890
Represents a file identifier across the system.

expenseId
required
string
Default: "##default"
Example: 64c777ddd3fcab07cfbb210c
Represents an expense identifier across the system.

workspaceId
required
string
Default: "##default"
Example: 64a687e29ae1f428e7ebe303
Represents a workspace identifier across the system.

Responses
200 OK
Response Schema: */*
string <byte>