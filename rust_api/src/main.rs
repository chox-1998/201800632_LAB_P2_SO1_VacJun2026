use axum::{
    extract::State,
    routing::post,
    Json,
    Router,
};
use serde::{Deserialize, Serialize};
use std::sync::Arc;

#[derive(Clone)]
struct AppState {
    go_service: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct Prediction {
    home_team: String,
    away_team: String,
    home_goals: i32,
    away_goals: i32,
    username: String,
    timestamp: String,
}

async fn prediction(
    State(state): State<Arc<AppState>>,
    Json(payload): Json<Prediction>,
) -> Result<String, String> {

    let client = reqwest::Client::new();

    let response = client
        .post(format!("{}/prediction", state.go_service))
        .json(&payload)
        .send()
        .await
        .map_err(|e| e.to_string())?;

    if response.status().is_success() {
        Ok("Prediction received".to_string())
    } else {
        Err("Go service returned error".to_string())
    }
}

#[tokio::main]
async fn main() {

    let state = Arc::new(AppState {
        go_service: "http://go-deployment1:8080".to_string(),
    });

    let app = Router::new()
        .route("/grpc-202012345", post(prediction))
        .with_state(state);

    let listener = tokio::net::TcpListener::bind("0.0.0.0:8080")
        .await
        .unwrap();

    println!("Listening on port 8080");

    axum::serve(listener, app)
        .await
        .unwrap();
}
