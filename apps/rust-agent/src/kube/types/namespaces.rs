use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize)]
struct NamespaceMetadata {
    name: String,
}

#[derive(Serialize, Deserialize)]
struct NamespaceItem {
    metadata: NamespaceMetadata,
}

#[derive(Serialize, Deserialize)]
pub struct NamespaceListResponse {
    items: Vec<NamespaceItem>,
}

impl NamespaceListResponse {
    pub fn as_string(&self) -> String {
        let namespace_names: Vec<String> = self
            .items
            .iter()
            .map(|item| item.metadata.name.clone())
            .collect();
        namespace_names.join(", ")
    }
}
