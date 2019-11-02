# Data Exchanges <img style="float: right" src="/images/exchanges.png" class="cloud-provider">

Exchanges are internal hooks to external apis, to either push data and update an external service, or pull data and update itself from some external service.

Example, use exchange to sync data creation call to Google Sheets. So on every row created using the POST API also creates a corresponding row in your google sheet.

!!! note "Google drive exchange YAML"
    ```yaml
    Exchanges:
    - Name: Task to excel sheet
      SourceAttributes:
        Name: todo
      SourceType: self
      TargetAttributes:
        sheetUrl: https://content-sheets.googleapis.com/v4/spreadsheets/1Ru-bDk3AjQotQj72k8SyxoOs84eXA1Y6sSPumBb3WSA/values/A1:append
        appKey: AIzaSyAC2xame4NShrzH9ZJeEpWT5GkySooa0XM
      TargetType: gsheet-append
      Attributes:
      - SourceColumn: "$self.description"
        TargetColumn: Task description
      - SourceColumn: self.schedule
        TargetColumn: Scheduled at
      Options:
        hasHeader: true
    ```

