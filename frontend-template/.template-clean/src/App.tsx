import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'

// Demo pages were removed by ./clean_out_template.sh.
//
// Add your pages under src/pages/ and register routes below. Route paths are
// basename-relative — main.tsx sets <BrowserRouter basename={getBasePath()}>
// from the moses-base-path meta tag (CHAT-pbup), so "/" resolves to
// /apps/<tenant>/<slug>/ when deployed through Moses. Use react-router
// <Link to="..."> for internal navigation, never raw absolute <a href="/...">.

function HomePlaceholder() {
  return (
    <section className="section">
      <h1>My App</h1>
      <p>
        Replace this placeholder: add pages under <code>src/pages/</code> and
        register routes in <code>src/App.tsx</code>.
      </p>
    </section>
  )
}

function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<HomePlaceholder />} />
      </Routes>
    </Layout>
  )
}

export default App
