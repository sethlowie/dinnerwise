import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TransportProvider } from "@connectrpc/connect-query";
import { RouterProvider } from "@tanstack/react-router";
import { transport } from "./transport";
import { ThemeProvider } from "./theme";
import { router } from "./router";

const queryClient = new QueryClient();

function App() {
  return (
    <ThemeProvider>
      <TransportProvider transport={transport}>
        <QueryClientProvider client={queryClient}>
          <RouterProvider router={router} />
        </QueryClientProvider>
      </TransportProvider>
    </ThemeProvider>
  );
}

export default App;
