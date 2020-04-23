export default function () {
  return {
    tables: {},
    token: localStorage.getItem("token"),
    showHiddenTables: true,
    selectedTable: null,
  }
}
