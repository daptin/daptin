export function domain(url) {
  let urlParser = document.createElement('a')
  urlParser.href = url
  return urlParser.hostname
}

export function count(arr) {
  return arr.length
}

export function prettyDate(date) {
  var a = new Date(date)
  return a.toDateString()
}

export function pluralize(time, label) {
  if (time === 1) {
    return time + label
  }

  return time + label + 's'
}
