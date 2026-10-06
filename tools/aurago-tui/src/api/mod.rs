use anyhow::{Context, Result};
use reqwest::{Client, ClientBuilder, Method, RequestBuilder, Response};
use serde::{Serialize, de::DeserializeOwned};
use std::sync::{Arc, Mutex};
use std::time::Duration;

pub mod auth;
pub mod sse;
pub mod types;
#[cfg(test)]
pub(crate) mod test_server;

#[derive(Debug, Clone)]
pub struct ApiClient {
    pub client: Client,
    pub base_url: String,
    pub session_cookie: Arc<Mutex<Option<String>>>,
}

impl ApiClient {
    pub fn new(base_url: &str, insecure: bool) -> Result<Self> {
        let mut builder = ClientBuilder::new()
            .cookie_store(false)
            .timeout(Duration::from_secs(30));
        if insecure {
            builder = builder.danger_accept_invalid_certs(true);
        }
        let client = builder.build().context("Failed to build HTTP client")?;
        let mut url = base_url.to_string();
        while url.ends_with('/') {
            url.pop();
        }
        Ok(Self {
            client,
            base_url: url,
            session_cookie: Arc::new(Mutex::new(None)),
        })
    }

    pub fn set_session_cookie(&self, cookie: String) {
        if let Ok(mut guard) = self.session_cookie.lock() {
            *guard = Some(cookie);
        }
    }

    pub fn get_session_cookie(&self) -> Option<String> {
        self.session_cookie.lock().ok().and_then(|g| g.clone())
    }

    pub async fn request<B, R>(&self, method: Method, path: &str, body: Option<&B>) -> Result<R>
    where
        B: Serialize,
        R: DeserializeOwned,
    {
        let req = self.build_request(method, path, body);
        let resp = Self::send_checked(req).await?;
        let data = resp
            .json::<R>()
            .await
            .context("Failed to decode JSON response")?;
        Ok(data)
    }

    pub async fn request_raw<B>(
        &self,
        method: Method,
        path: &str,
        body: Option<&B>,
    ) -> Result<reqwest::Response>
    where
        B: Serialize,
    {
        let req = self.build_request(method, path, body);
        Self::send_checked(req).await
    }

    pub async fn request_empty<B>(&self, method: Method, path: &str, body: Option<&B>) -> Result<()>
    where
        B: Serialize,
    {
        let req = self.build_request(method, path, body);
        Self::send_checked(req).await?;
        Ok(())
    }

    /// Sends a request and returns the status with the JSON body (or the body
    /// text as a JSON string), without turning a non-2xx answer into an error.
    pub async fn request_json_with_status(&self, method: Method, path: &str) -> Result<(reqwest::StatusCode, serde_json::Value)> {
        let req = self.build_request(method, path, None::<&()>);
        let resp = req.send().await.context("HTTP request failed")?;
        let status = resp.status();
        let text = resp.text().await.unwrap_or_default();
        let value = serde_json::from_str(&text).unwrap_or(serde_json::Value::String(text));
        Ok((status, value))
    }

    fn build_request<B>(&self, method: Method, path: &str, body: Option<&B>) -> RequestBuilder
    where
        B: Serialize,
    {
        let url = format!("{}{}", self.base_url, path);
        let mut req = self.client.request(method, &url);
        req = req.header("Origin", &self.base_url);
        if let Some(b) = body {
            req = req.json(b);
        }
        if let Some(cookie) = self.get_session_cookie() {
            req = req.header("Cookie", cookie);
        }
        req
    }

    async fn send_checked(req: RequestBuilder) -> Result<Response> {
        let resp = req.send().await.context("HTTP request failed")?;
        let status = resp.status();
        if !status.is_success() {
            let text = resp.text().await.unwrap_or_default();
            anyhow::bail!("HTTP {}: {}", status, text);
        }
        Ok(resp)
    }

    pub fn sse_url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}
