//! A canned HTTP server for API tests: it answers each "METHOD path?query"
//! with a fixed status and body and records the requests it saw. Unknown
//! requests get 404 with Go's plain-text body, as an older AuraGo would send.

use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::{Arc, Mutex};

pub(crate) struct Route {
    pub method: &'static str,
    pub target: &'static str,
    pub status: u16,
    pub body: &'static str,
}

pub(crate) struct CannedServer {
    pub base_url: String,
    pub requests: Arc<Mutex<Vec<String>>>,
}

impl CannedServer {
    pub fn seen(&self) -> Vec<String> {
        self.requests.lock().map(|r| r.clone()).unwrap_or_default()
    }
}

pub(crate) fn start(routes: Vec<Route>) -> CannedServer {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind canned server");
    let base_url = format!("http://{}", listener.local_addr().expect("local addr"));
    let requests = Arc::new(Mutex::new(Vec::new()));
    let seen = Arc::clone(&requests);
    std::thread::spawn(move || {
        for stream in listener.incoming() {
            let Ok(mut stream) = stream else { break };
            let Ok(clone) = stream.try_clone() else { continue };
            let mut reader = BufReader::new(clone);
            let mut request_line = String::new();
            if reader.read_line(&mut request_line).is_err() {
                continue;
            }
            let mut parts = request_line.split_whitespace();
            let method = parts.next().unwrap_or_default().to_string();
            let target = parts.next().unwrap_or_default().to_string();
            let mut length = 0usize;
            loop {
                let mut header = String::new();
                if reader.read_line(&mut header).unwrap_or(0) == 0 {
                    break;
                }
                let header = header.trim_end();
                if header.is_empty() {
                    break;
                }
                if let Some(value) = header.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = value.trim().parse().unwrap_or(0);
                }
            }
            let mut body = vec![0; length];
            let _ = reader.read_exact(&mut body);
            if let Ok(mut log) = seen.lock() {
                log.push(format!("{} {}", method, target));
            }
            let (status, body, content_type) = routes
                .iter()
                .find(|r| r.method == method && r.target == target)
                .map(|r| (r.status, r.body, "application/json"))
                .unwrap_or((404, "404 page not found\n", "text/plain; charset=utf-8"));
            let response = format!(
                "HTTP/1.1 {} Canned\r\nContent-Type: {}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                status,
                content_type,
                body.len(),
                body
            );
            let _ = stream.write_all(response.as_bytes());
        }
    });
    CannedServer { base_url, requests }
}
