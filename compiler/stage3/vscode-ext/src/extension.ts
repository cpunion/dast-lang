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

    // Get server path from configuration or use default
    const config = vscode.workspace.getConfiguration('dast');
    let serverPath = config.get<string>('server.path') || '';

    if (!serverPath) {
        // Try to find dast-lsp in common locations
        const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
        if (workspaceFolder) {
            // Look relative to workspace
            const localPath = path.join(workspaceFolder.uri.fsPath, 'compiler/stage3/lsp/dast-lsp');
            serverPath = localPath;
        }

        // Fallback to PATH
        if (!serverPath) {
            serverPath = 'dast-lsp';
        }
    }

    console.log(`Using LSP server: ${serverPath}`);

    // Server options
    const serverOptions: ServerOptions = {
        command: serverPath,
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
        vscode.window.showErrorMessage(`Failed to start Dast Language Server: ${error.message}`);
    });

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
