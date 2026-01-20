"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;
const path = __importStar(require("path"));
const vscode = __importStar(require("vscode"));
const node_1 = require("vscode-languageclient/node");
let client;
function activate(context) {
    console.log('Dast Language extension is activating...');
    const config = vscode.workspace.getConfiguration('dast');
    // Get workspace root
    const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
    const workspaceRoot = workspaceFolder?.uri.fsPath || '';
    // Get LSP wrapper script path
    let lspPath = config.get('server.path') || '';
    if (!lspPath && workspaceRoot) {
        // Try to find dast-lsp.sh in workspace
        const localPath = path.join(workspaceRoot, 'compiler/stage3/lsp/dast-lsp.sh');
        lspPath = localPath;
    }
    if (!lspPath) {
        lspPath = 'dast-lsp'; // Fallback to PATH
    }
    console.log(`Using Dast LSP: ${lspPath}`);
    // Server options - launches the wrapper script
    const serverOptions = {
        command: lspPath,
        args: [workspaceRoot],
        transport: node_1.TransportKind.stdio,
    };
    // Client options
    const clientOptions = {
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
    client = new node_1.LanguageClient('dast', 'Dast Language Server', serverOptions, clientOptions);
    // Start the client
    client.start().then(() => {
        console.log('Dast Language Server started');
    }).catch((error) => {
        console.error('Failed to start Dast Language Server:', error);
        vscode.window.showErrorMessage(`Failed to start Dast Language Server: ${error.message}\n` +
            `Make sure the wrapper script exists and is executable: ${lspPath}`);
    });
    // Register restart command
    context.subscriptions.push(vscode.commands.registerCommand('dast.restartServer', async () => {
        if (client) {
            await client.stop();
            await client.start();
        }
    }));
    context.subscriptions.push({
        dispose: () => {
            if (client) {
                client.stop();
            }
        }
    });
}
function deactivate() {
    if (!client) {
        return undefined;
    }
    return client.stop();
}
//# sourceMappingURL=extension.js.map