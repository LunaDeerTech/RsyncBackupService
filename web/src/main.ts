import { createApp } from "vue"

import App from "./App.vue"
import { createRouter } from "./router"
import "./styles/reset.css"
import "./styles/application.css"
import "./styles/tokens.css"
import "./styles/theme-light.css"
import "./styles/theme-dark.css"

const app = createApp(App)

app.use(createRouter())
app.mount("#app")