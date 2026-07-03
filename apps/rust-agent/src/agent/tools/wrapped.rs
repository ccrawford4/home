use rig::completion::ToolDefinition;
use rig::tool::Tool;

use super::portfolio_api_search::{ModelError, PortfolioAPISearch, PortfolioAPISearchArgs};

pub struct WrappedPortfolioAPISearch;

impl Tool for WrappedPortfolioAPISearch {
    const NAME: &'static str = "web_search";
    type Error = ModelError;
    type Args = PortfolioAPISearchArgs;
    type Output = String;

    async fn definition(&self, _prompt: String) -> ToolDefinition {
        PortfolioAPISearch.definition(_prompt).await
    }

    async fn call(&self, args: Self::Args) -> Result<Self::Output, Self::Error> {
        PortfolioAPISearch.call(args).await
    }
}
