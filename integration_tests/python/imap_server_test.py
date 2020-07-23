from imapclient import IMAPClient

server = IMAPClient('localhost', port=1143, use_uid=True, ssl=False)
server.login('test@localhost', 'test')
select_info = server.select_folder('INBOX')

print('%d messages in INBOX' % select_info[b'EXISTS'])

messages = server.search(['FROM', 'best-friend@domain.com'])

print("%d messages from our best friend" % len(messages))

for msgid, data in server.fetch([27], ['ENVELOPE']).items():
    envelope = data[b'ENVELOPE']
    print('ID #%d: "%s" received %s' % (msgid, envelope.subject.decode(), envelope.date))
    print('ID #%d: "%s"' % (msgid, data))

server.logout()
