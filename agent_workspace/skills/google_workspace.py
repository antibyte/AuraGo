import os
import json
import sys
import datetime
from google.oauth2.credentials import Credentials
from google_auth_oauthlib.flow import InstalledAppFlow
from google.auth.transport.requests import Request
from googleapiclient.discovery import build
from urllib.parse import urlparse, parse_qs

# Allow http for localhost during development
os.environ['OAUTHLIB_INSECURE_TRANSPORT'] = '1'

# Scopes covering Gmail, Calendar, Drive, and Documents
SCOPES = [
    'https://www.googleapis.com/auth/gmail.modify',
    'https://www.googleapis.com/auth/calendar.readonly',
    'https://www.googleapis.com/auth/drive.readonly',
    'https://www.googleapis.com/auth/documents'
]

def get_secrets_path(creds_dir):
    for name in ["client_secrets.json", "client_secret.json"]:
        path = os.path.join(creds_dir, name)
        if os.path.exists(path):
            return path
    return None

def get_credentials(creds_dir, vault_secrets=None):
    token_path = os.path.join(creds_dir, 'token.json')
    creds = None
    
    # Try using vault secrets if provided
    if vault_secrets and 'google_workspace_token' in vault_secrets:
        token_data = json.loads(vault_secrets['google_workspace_token'])
        creds = Credentials.from_authorized_user_info(token_data, SCOPES)
    elif os.path.exists(token_path):
        creds = Credentials.from_authorized_user_file(token_path, SCOPES)
        
    if not creds or not creds.valid:
        try:
            if creds and creds.expired and creds.refresh_token:
                creds.refresh(Request())
            else:
                raise Exception("Force re-auth")
        except Exception:
            # Try fetching client config from vault or disk
            client_config = None
            if vault_secrets and 'google_workspace_client_secret' in vault_secrets:
                client_config = json.loads(vault_secrets['google_workspace_client_secret'])
            else:
                secrets_path = get_secrets_path(creds_dir)
                if not secrets_path:
                    return {'error': f'No client_secret.json or client_secrets.json found in {creds_dir}'}
                with open(secrets_path, 'r') as f:
                    client_config = json.load(f)
            
            flow = InstalledAppFlow.from_client_config(
                client_config, SCOPES, redirect_uri='http://localhost:8080/'
            )
            auth_url, _ = flow.authorization_url(prompt='consent', access_type='offline')
            return {
                'error': 'AUTH_REQUIRED',
                'auth_url': auth_url,
                'message': 'Visit the URL to authorize, then copy the redirected localhost URL and use action="submit_auth_url".'
            }
        
        # If we successfully refreshed, we might want to return the new token 
        # so the Go caller can save it back to the Vault.
        # For now, we just return the creds and assume the caller knows how to handle it.
        
    return creds

def submit_auth_url(auth_url, creds_dir=None, vault_secrets=None):
    if creds_dir is None:
        creds_dir = os.path.dirname(os.path.abspath(__file__))
    
    # Parse the authorization code from the URL
    parsed = urlparse(auth_url)
    params = parse_qs(parsed.query)
    code = params.get('code', [None])[0]
    if not code:
        return {'error': 'Authorization code not found in URL'}
    
    # Get client config
    client_config = None
    if vault_secrets and 'google_workspace_client_secret' in vault_secrets:
        client_config = json.loads(vault_secrets['google_workspace_client_secret'])
    else:
        secrets_path = get_secrets_path(creds_dir)
        if not secrets_path:
            return {'error': f'No client_secret.json or client_secrets.json found in {creds_dir}'}
        with open(secrets_path, 'r') as f:
            client_config = json.load(f)
            
    # Perform token exchange
    flow = InstalledAppFlow.from_client_config(client_config, SCOPES)
    redirect_uri = 'http://localhost:8080/'
    flow.redirect_uri = redirect_uri
    flow.fetch_token(code=code)
    creds = flow.credentials
    
    return {
        'status': 'success', 
        'message': 'Authentication successful. Token returned in vault_update.',
        'vault_update': {'google_workspace_token': creds.to_json()}
    }

def read_emails(max_results=5, creds_dir=None, vault_secrets=None):
    if creds_dir is None:
        creds_dir = os.path.dirname(os.path.abspath(__file__))
    creds = get_credentials(creds_dir, vault_secrets)
    if isinstance(creds, dict) and 'error' in creds:
        return creds
    service = build('gmail', 'v1', credentials=creds)
    result = service.users().messages().list(userId='me', maxResults=int(max_results)).execute()
    messages = result.get('messages', [])
    emails = []
    for msg in messages:
        msg_detail = service.users().messages().get(userId='me', id=msg['id'], format='metadata').execute()
        headers = msg_detail.get('payload', {}).get('headers', [])
        subject = next((h['value'] for h in headers if h['name'] == 'Subject'), 'No Subject')
        sender = next((h['value'] for h in headers if h['name'] == 'From'), 'Unknown')
        date = next((h['value'] for h in headers if h['name'] == 'Date'), 'Unknown')
        snippet = msg_detail.get('snippet', '')

        # Sandwich Method: Wrap untrusted content in XML tags
        subject_wrapped = f"<external_data>{subject}</external_data>"
        snippet_wrapped = f"<external_data>{snippet}</external_data>"

        emails.append({
            'id': msg['id'],
            'thread_id': msg.get('threadId'),
            'subject': subject_wrapped,
            'from': sender,
            'date': date,
            'snippet': snippet_wrapped
        })
    return {'emails': emails}

def get_events(max_results=10, time_min=None, creds_dir=None, vault_secrets=None):
    if creds_dir is None:
        creds_dir = os.path.dirname(os.path.abspath(__file__))
    creds = get_credentials(creds_dir, vault_secrets)
    if isinstance(creds, dict) and 'error' in creds:
        return creds
    service = build('calendar', 'v3', credentials=creds)
    if not time_min:
        time_min = datetime.datetime.utcnow().isoformat() + 'Z'
    events_result = service.events().list(
        calendarId='primary', timeMin=time_min,
        maxResults=int(max_results), singleEvents=True,
        orderBy='startTime').execute()
    events = events_result.get('items', [])

    # Sandwich Method: Wrap untrusted content in XML tags
    for event in events:
        if 'summary' in event:
            event['summary'] = f"<external_data>{event['summary']}</external_data>"
        if 'description' in event:
            event['description'] = f"<external_data>{event['description']}</external_data>"

    return {'events': events}

def search_drive(query='', max_results=5, creds_dir=None, vault_secrets=None):
    if creds_dir is None:
        creds_dir = os.path.dirname(os.path.abspath(__file__))
    creds = get_credentials(creds_dir, vault_secrets)
    if isinstance(creds, dict) and 'error' in creds:
        return creds
    service = build('drive', 'v3', credentials=creds)
    results = service.files().list(
        q=query, pageSize=int(max_results), 
        fields="nextPageToken, files(id, name, mimeType, webViewLink, modifiedTime)").execute()
    items = results.get('files', [])

    # Sandwich Method: Wrap untrusted content in XML tags
    for item in items:
        if 'name' in item:
            item['name'] = f"<external_data>{item['name']}</external_data>"

    return {'files': items}

def _extract_doc_content(elements):
    """Helper to extract text from Google Docs structural elements."""
    text = ""
    for element in elements:
        if 'paragraph' in element:
            for run in element.get('paragraph').get('elements'):
                text += run.get('textRun', {}).get('content', '')
        elif 'table' in element:
            for row in element.get('table').get('tableRows'):
                for cell in row.get('tableCells'):
                    text += _extract_doc_content(cell.get('content'))
        elif 'tableOfContents' in element:
            text += _extract_doc_content(element.get('tableOfContents').get('content'))
    return text

def read_document(document_id, creds_dir=None, vault_secrets=None):
    if creds_dir is None:
        creds_dir = os.path.dirname(os.path.abspath(__file__))
    creds = get_credentials(creds_dir, vault_secrets)
    if isinstance(creds, dict) and 'error' in creds:
        return creds
    service = build('docs', 'v1', credentials=creds)
    doc = service.documents().get(documentId=document_id).execute()
    content = _extract_doc_content(doc.get('body').get('content'))

    # Sandwich Method: Wrap untrusted content in XML tags
    title_wrapped = f"<external_data>{doc.get('title')}</external_data>"
    content_wrapped = f"<external_data>{content}</external_data>"

    return {
        'document_id': document_id,
        'title': title_wrapped,
        'content': content_wrapped
    }

def write_document(document_id=None, title='Untitled', text='', append=True, creds_dir=None, vault_secrets=None):
    if creds_dir is None:
        creds_dir = os.path.dirname(os.path.abspath(__file__))
    creds = get_credentials(creds_dir, vault_secrets)
    if isinstance(creds, dict) and 'error' in creds:
        return creds
    service = build('docs', 'v1', credentials=creds)
    
    # If the credentials were refreshed, we need to return the updated token
    # to be stored back in the vault.
    token_update = None
    if creds.expired and creds.refresh_token:
        # Note: In a real scenario, we'd check if it was actually refreshed
        pass 

    # (Implementation continues below...)
    
    if not document_id:
        # Create new document
        doc = service.documents().create(body={'title': title}).execute()
        document_id = doc.get('documentId')
        # Initial text insertion
        requests = [{'insertText': {'location': {'index': 1}, 'text': text}}]
        service.documents().batchUpdate(documentId=document_id, body={'requests': requests}).execute()
        return {'status': 'success', 'action': 'created', 'document_id': document_id}

    # Updating existing document
    doc = service.documents().get(documentId=document_id).execute()
    requests = []
    
    if not append:
        # Overwrite: delete everything first
        end_index = doc.get('body').get('content')[-1].get('endIndex')
        if end_index > 2: # Can only delete if there is content (index 1 is start, doc always ends with newline at end_index-1)
            requests.append({'deleteContentRange': {'range': {'startIndex': 1, 'endIndex': end_index - 1}}})
        requests.append({'insertText': {'location': {'index': 1}, 'text': text}})
    else:
        # Append: find end index
        end_index = doc.get('body').get('content')[-1].get('endIndex')
        requests.append({'insertText': {'location': {'index': end_index - 1}, 'text': text}})

    service.documents().batchUpdate(documentId=document_id, body={'requests': requests}).execute()
    return {'status': 'success', 'action': 'updated', 'document_id': document_id}

def main():
    args = {}
    debug_info = {}
    
    # Force binary mode on Windows to avoid newline mangling
    if sys.platform == 'win32':
        import msvcrt
        msvcrt.setmode(sys.stdin.fileno(), os.O_BINARY)

    # Try Stdin first
    try:
        stdin_data = sys.stdin.read().strip()
        if stdin_data:
            args = json.loads(stdin_data)
            debug_info['source'] = 'stdin'
    except Exception as e:
        debug_info['stdin_error'] = str(e)
    
    # Fallback to Argv (useful for manual testing)
    if not args and len(sys.argv) > 1:
        try:
            args = json.loads(sys.argv[1])
            debug_info['source'] = 'argv'
        except Exception as e:
            debug_info['argv_error'] = str(e)

    if not args:
        # Check if we got something but it wasn't a dict
        if args is None:
            args = {}
        else:
            print(json.dumps({'error': 'No input provided', 'debug': debug_info}))
            return

    action = args.get('action') or args.get('operation')
    if action == 'list_emails':
        action = 'read_emails'
        
    creds_dir = os.path.dirname(os.path.abspath(__file__))
    vault_secrets = args.get('vault_secrets')
    result = None
    
    if action == 'get_events':
        result = get_events(args.get('max_results', 10), args.get('time_min'), creds_dir, vault_secrets)
    elif action == 'read_emails':
        result = read_emails(args.get('max_results', 5), creds_dir, vault_secrets)
    elif action == 'search_drive':
        result = search_drive(args.get('query', ''), args.get('max_results', 5), creds_dir, vault_secrets)
    elif action == 'read_document':
        doc_id = args.get('document_id')
        if not doc_id:
            result = {'error': 'Missing document_id parameter'}
        else:
            result = read_document(doc_id, creds_dir, vault_secrets)
    elif action == 'write_document':
        result = write_document(
            document_id=args.get('document_id'),
            title=args.get('title', 'Untitled'),
            text=args.get('text', ''),
            append=args.get('append', True) if isinstance(args.get('append'), bool) else True,
            creds_dir=creds_dir,
            vault_secrets=vault_secrets
        )
    elif action == 'submit_auth_url':
        auth_url = args.get('auth_url')
        if not auth_url:
            result = {'error': 'Missing auth_url parameter'}
        else:
            result = submit_auth_url(auth_url, creds_dir, vault_secrets)
    else:
        result = {'error': f'Unknown action: {action}', 'received_args': args, 'debug': debug_info}
    
    # Ensure stdout is UTF-8 for Windows consistency
    sys.stdout.reconfigure(encoding='utf-8')
    print(json.dumps(result))

if __name__ == '__main__':
    main()
