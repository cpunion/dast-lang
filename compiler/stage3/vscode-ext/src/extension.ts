import * as path from 'path';
import * as vscode from 'vscode';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind
} from 'vscode-languageclient/node';

let client: LanguageClient;

export function activate(context: vscode.ExtensionContext) {
    console.log('Dast Language extension is activating...');

    const config = vscode.workspace.getConfiguration('dast');

    // Get workspace root
    const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
    const workspaceRoot = workspaceFolder?.uri.fsPath || '';

    // Get LSP wrapper script path
    let lspPath = config.get<string>('server.path') || '';

    if (!lspPath && workspaceRoot) {
        // Try to find dast-lsp.sh in workspace
        const localPath = path.join(workspaceRoot, 'compiler/stage3/lsp/dast-lsp.sh');
        lspPath = localPath;
    }

    if (!lspPath) {
        lspPath = 'dast-lsp';  // Fallback to PATH
    }

    console.log(`Using Dast LSP: ${lspPath}`);

    // Server options - launches the wrapper script
    const serverOptions: ServerOptions = {
        command: lspPath,
        args: [workspaceRoot],
        transport: TransportKind.stdio,
    };

    // Client options
    const clientOptions: LanguageClientOptions = {
        documentSelector: [
            { scheme: 'file', language: 'dast' }
        ],
        synchronize: {
            fileEvents: vscode.workspace.createFileSystemWatcher('**/*.dast')
        },
        outputChannelName: 'Dast Language Server',
        initializationOptions: {
            workspaceRoot: workspaceRoot,
        },
    };

    // Create and start the client
    client = new LanguageClient(
        'dast',
        'Dast Language Server',
        serverOptions,
        clientOptions
    );

    // Start the client
    client.start().then(() => {
        console.log('Dast Language Server started');
    }).catch((error) => {
        console.error('Failed to start Dast Language Server:', error);
        vscode.window.showErrorMessage(
            `Failed to start Dast Language Server: ${error.message}\n` +
            `Make sure the wrapper script exists and is executable: ${lspPath}`
        );
    });

    // Register restart command
    context.subscriptions.push(
        vscode.commands.registerCommand('dast.restartServer', async () => {
            if (client) {
                await client.stop();
                await client.start();
            }
        })
    );

    context.subscriptions.push({
        dispose: () => {
            if (client) {
                client.stop();
            }
        }
    });
}

export function deactivate(): Thenable<void> | undefined {
    if (!client) {
        return undefined;
    }
    return client.stop();
}
