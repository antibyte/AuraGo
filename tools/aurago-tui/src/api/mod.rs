use anyhow::{Context, Result};
use reqwest::{Client, ClientBuilder, Method};
use serde::{de::DeserializeOwned, Serialize};
use std::time::Duration;

pub mod auth;
pub mod sse;
pub mod types;

#[derive(Debug, Clone)]
pub struct ApiClient {
    pub client: Client,
    pub base_url: String,
}

impl ApiClient {
    pub fn new(base_url: &str) -> Result<Self> {
        let client = ClientBuilder::new()
            .cookie_store(true)
            .timeout(Duration::from_secs(30))
            .build()
            .context("Failed to build HTTP client")?;
        let mut url = base_url.to_string();
        while url.ends_with('/') {
            url.pop();
        }
        Ok(Self { client, base_url: url })
    }

    pub async fn request<B, R>(&self, method: Method, path: &str, body: Option<&B>) -> Result<R>
    where
        B: Serialize,
        R: DeserializeOwned,
    {
        let url = format!("{}{}", self.base_url, path);
        let mut req = self.client.request(method, &url);
        if let Some(b) = body {
            req = req.json(b);
        }
        let resp = req.send().await.context("HTTP request failed")?;
        let status = resp.status();
        if !status.is_success() {
            let text = resp.text().await.unwrap_or_default();
            anyhow::bail!("HTTP {}: {}", status, text);
        }
        let data = resp.json::<R>().await.context("Failed to decode JSON response")?;
        Ok(data)
    }

    pub async fn request_empty<B>(&self, method: Method, path: &str, body: Option<&B>) -> Result<()>
    where
        B: Serialize,
    {
        let url = format!("{}{}", self.base_url, path);
        let mut req = self.client.request(method, &url);
        if let Some(b) = body {
            req = req.json(b);
        }
        let resp = req.send().await.context("HTTP request failed")?;
        let status = resp.status();
        if !status.is_success() {
            let text = resp.text().await.unwrap_or_default();
            anyhow::bail!("HTTP {}: {}", status, text);
        }
        Ok(())
    }

    pub fn sse_url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }
}
