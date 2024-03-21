using System;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.AspNetCore.Hosting;
using Microsoft.Extensions.Hosting;

namespace DotNetTestApp
{
    public class Program
    {
        public static async Task Main(string[] args)
        {
            var serverTask = CreateHostBuilder(args).Build().RunAsync();
            var clientTask = RunClientAsync();

            await Task.WhenAny(serverTask, clientTask);
        }

        public static IHostBuilder CreateHostBuilder(string[] args) =>
            Host.CreateDefaultBuilder(args)
                .ConfigureWebHostDefaults(webBuilder =>
                {
                    webBuilder.UseStartup<Startup>().UseUrls("http://*:3000"); // Listen on port 3000
                });

        private static async Task RunClientAsync()
        {
            var client = new HttpClient();
            while (true)
            {
                try
                {
                    var response = await client.GetAsync("http://localhost:3000");
                    Console.WriteLine($"Received response: {response.StatusCode}");
                }
                catch (Exception ex)
                {
                    Console.WriteLine($"Error: {ex.Message}");
                }

                await Task.Delay(1000); // Wait for 1 second
            }
        }
    }
}
