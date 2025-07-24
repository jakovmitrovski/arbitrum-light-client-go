import json
import numpy as np
import matplotlib.pyplot as plt
import pandas as pd
from typing import Dict, List, Tuple, Optional
from matplotlib import rc
import os
from pathlib import Path

rc('text', usetex=True)
rc(
    'font',
    family='serif',
    serif=['Computer Modern Roman'],
    monospace=['Computer Modern Typewriter'],
    size=12
)

def load_csv_data(file_path: str) -> pd.DataFrame:
    """Load CSV data from measurement files"""
    try:
        df = pd.read_csv(file_path)
        return df
    except FileNotFoundError:
        print(f"Warning: Could not find file {file_path}")
        return pd.DataFrame()

def calculate_stats_by_provers(df: pd.DataFrame, metric: str) -> Dict[int, Tuple[float, float, List[float]]]:
    """Calculate mean, std, and all values for each number of provers"""
    stats = {}
    
    if 'num_provers' not in df.columns:
        # For consensus/execution data without num_provers column
        if len(df) > 0:
            mean_val = df[metric].mean()
            std_val = df[metric].std()
            all_values = df[metric].tolist()
            stats[1] = (mean_val, std_val, all_values)
        return stats
    
    # Handle data with num_provers column
    for num_provers in sorted(df['num_provers'].unique()):
        subset = df[df['num_provers'] == num_provers]
        if len(subset) > 0:
            mean_val = subset[metric].mean()
            std_val = subset[metric].std()
            all_values = subset[metric].tolist()
            stats[num_provers] = (mean_val, std_val, all_values)
    
    return stats

def plot_metric_with_std(df: pd.DataFrame, metric: str, title: str, ylabel: str, 
                        colors: List[str] = None, save_path: str = None):
    """Plot a metric with standard deviation for different numbers of provers"""
    if len(df) == 0:
        print(f"No data available for {metric}")
        return
    
    stats = calculate_stats_by_provers(df, metric)
    if not stats:
        print(f"No valid data for {metric}")
        return
    
    plt.figure(figsize=(10, 6))
    
    if 'num_provers' in df.columns and len(df['num_provers'].unique()) > 1:
        provers_list = sorted(stats.keys())
        x_positions = np.arange(len(provers_list))
        
        means = [stats[p][0] for p in provers_list]
        stds = [stats[p][1] for p in provers_list]
        
        # Use different colors for multiple provers
        colors = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b']
        for i, (prover_count, mean_val, std_val) in enumerate(zip(provers_list, means, stds)):
            display_provers = prover_count + 1
            plt.errorbar([i], [mean_val], yerr=[std_val], fmt='o-', 
                        capsize=5, capthick=2, linewidth=2, markersize=8,
                        color=colors[i % len(colors)], label=f'{display_provers} provers')
        
        # Display prover numbers as 1, 2, 3... instead of 0, 1, 2...
        display_provers = [p + 1 for p in provers_list]
        plt.xticks(x_positions, [f'{p} provers' for p in display_provers])
        plt.xlabel('Number of Provers')
        
    else:
        # For consensus/execution data or single prover tournament data - use blue
        mean_val, std_val, all_values = stats[list(stats.keys())[0]]
        plt.errorbar([0], [mean_val], yerr=[std_val], fmt='o-',
                    capsize=5, capthick=2, linewidth=2, markersize=8,
                    color='#1f77b4', label=f'{metric.replace("_", " ").title()}')
        plt.xticks([0], ['Single Measurement'])
        plt.xlabel('Measurement Type')
    
    plt.ylabel(ylabel)
    plt.title(title)
    plt.grid(True, alpha=0.3)
    plt.legend()
    
    if save_path:
        plt.savefig(save_path, bbox_inches='tight', dpi=300)
        print(f"Saved plot to {save_path}")
    
    #plt.show()

def plot_all_metrics_for_type(data_type: str, df: pd.DataFrame, output_dir: str):
    """Plot all metrics for a specific data type (consensus, execution, or tournament)"""
    if len(df) == 0:
        print(f"No data available for {data_type}")
        return
    
    # Create output directory
    os.makedirs(output_dir, exist_ok=True)
    
    # Define metrics to plot based on data type
    if data_type == 'tournament':
        # For tournament, skip individual plots - we only want block-based plots
        pass
    else:  # consensus or execution - will be handled in main function
        pass

def plot_combined_oracle_metrics(consensus_df: pd.DataFrame, execution_df: pd.DataFrame, output_dir: str):
    """Create a combined plot showing all metrics for consensus and execution oracles together"""
    if len(consensus_df) == 0 and len(execution_df) == 0:
        print("No consensus or execution data available")
        return
    
    # Debug: Print available columns
    print(f"Consensus columns: {list(consensus_df.columns)}")
    print(f"Execution columns: {list(execution_df.columns)}")
    
    # Define metrics to plot (without oracle type prefix)
    metrics = [
        ('oracle_time_ms', 'Oracle Time (s)', 'oracle'),
        ('cpu_percent', 'CPU Usage (percent)', 'cpu'),
        ('memory_bytes', 'Memory Usage (MB)', 'memory'),
        ('network_bytes_in', 'Network Bytes In (MB)', 'network_in'),
        ('network_bytes_out', 'Network Bytes Out (MB)', 'network_out')
    ]
    
    # Create figure with subplots
    fig, axes = plt.subplots(2, 3, figsize=(18, 12))
    fig.suptitle('Oracle Measurements - Consensus vs Execution', fontsize=16, fontweight='bold')
    
    # Flatten axes for easier indexing
    axes = axes.flatten()
    
    for i, (metric, title, metric_type) in enumerate(metrics):
        ax = axes[i]
        
        # Plot consensus data if available
        if len(consensus_df) > 0:
            # Handle oracle time with prefix, other metrics without prefix
            if metric == 'oracle_time_ms':
                column_name = 'consensus_oracle_time_ms'
            else:
                column_name = metric
                
            if column_name in consensus_df.columns:
                consensus_values = consensus_df[column_name].values
                consensus_iterations = consensus_df['iteration'].values
                
                # Convert units for better readability
                if metric == 'oracle_time_ms':
                    consensus_values = consensus_values / 1000  # Convert to seconds
                elif metric == 'memory_bytes':
                    consensus_values = consensus_values / (1024 * 1024)  # Convert to MB
                elif metric in ['network_bytes_in', 'network_bytes_out']:
                    consensus_values = consensus_values / (1024 * 1024)  # Convert to MB
                
                # Plot consensus data in blue
                ax.plot(consensus_iterations, consensus_values, 'o-', color='#1f77b4', 
                       linewidth=2, markersize=6, label='Consensus')
                print(f"Plotted consensus {metric} with {len(consensus_values)} values")
            else:
                print(f"Consensus {column_name} not found in columns: {list(consensus_df.columns)}")
        else:
            print(f"Consensus {metric} not found in columns: {list(consensus_df.columns)}")
        
        # Plot execution data if available
        if len(execution_df) > 0:
            # Handle oracle time with prefix, other metrics without prefix
            if metric == 'oracle_time_ms':
                column_name = 'execution_oracle_time_ms'
            else:
                column_name = metric
                
            if column_name in execution_df.columns:
                execution_values = execution_df[column_name].values
                execution_iterations = execution_df['iteration'].values
                
                # Convert units for better readability
                if metric == 'oracle_time_ms':
                    execution_values = execution_values / 1000  # Convert to seconds
                elif metric == 'memory_bytes':
                    execution_values = execution_values / (1024 * 1024)  # Convert to MB
                elif metric in ['network_bytes_in', 'network_bytes_out']:
                    execution_values = execution_values / (1024 * 1024)  # Convert to MB
                
                # Plot execution data in red
                ax.plot(execution_iterations, execution_values, 's-', color='#d62728', 
                       linewidth=2, markersize=6, label='Execution')
                print(f"Plotted execution {metric} with {len(execution_values)} values")
            else:
                print(f"Execution {column_name} not found in columns: {list(execution_df.columns)}")
        else:
            print(f"Execution {metric} not found in columns: {list(execution_df.columns)}")
        
        ax.set_title(title, fontweight='bold')
        ax.set_xlabel('Iteration')
        ax.set_ylabel(title.split('(')[1].split(')')[0] if '(' in title else 'Value')
        ax.grid(True, alpha=0.3)
        ax.legend()
        
        # Set appropriate Y-axis range based on metric type
        if 'CPU Usage' in title:
            # CPU percentage: 0-100%
            ax.set_ylim(0, 100)
        elif 'Memory Usage' in title:
            # Memory in MB: 0-15 GB (15000 MB)
            ax.set_ylim(0, 15000)
        elif 'Network Bytes In' in title:
            # Network in MB: 0-20 MB
            ax.set_ylim(0, 20)
        elif 'Network Bytes Out' in title:
            # Network out in MB: 0-3 MB
            ax.set_ylim(0, 3)
        elif 'Oracle Time' in title:
            # Time in seconds: 0-30 seconds
            ax.set_ylim(0, 30)
        
        # Recursive dynamic Y-axis adjustment for low values
        all_values = []
        if len(consensus_df) > 0:
            # Handle oracle time with prefix, other metrics without prefix
            if metric == 'oracle_time_ms':
                column_name = 'consensus_oracle_time_ms'
            else:
                column_name = metric
                
            if column_name in consensus_df.columns:
                consensus_values = consensus_df[column_name].values
                if metric == 'oracle_time_ms':
                    consensus_values = consensus_values / 1000
                elif metric == 'memory_bytes':
                    consensus_values = consensus_values / (1024 * 1024)
                elif metric in ['network_bytes_in', 'network_bytes_out']:
                    consensus_values = consensus_values / (1024 * 1024)
                all_values.extend(consensus_values)
        
        if len(execution_df) > 0:
            # Handle oracle time with prefix, other metrics without prefix
            if metric == 'oracle_time_ms':
                column_name = 'execution_oracle_time_ms'
            else:
                column_name = metric
                
            if column_name in execution_df.columns:
                execution_values = execution_df[column_name].values
                if metric == 'oracle_time_ms':
                    execution_values = execution_values / 1000
                elif metric == 'memory_bytes':
                    execution_values = execution_values / (1024 * 1024)
                elif metric in ['network_bytes_in', 'network_bytes_out']:
                    execution_values = execution_values / (1024 * 1024)
                all_values.extend(execution_values)
        
        if len(all_values) > 0:
            data_min = np.min(all_values)
            data_max = np.max(all_values)
            data_range = data_max - data_min
            
            # Include some padding for better visualization
            data_range_with_padding = data_range * 1.2  # Add 20% padding
            
            # Keep zooming until we get a reasonable data-to-range ratio
            max_iterations = 3  # Reduce from 5 to 3
            iteration = 0
            
            while iteration < max_iterations:
                current_ylim = ax.get_ylim()
                current_range = current_ylim[1] - current_ylim[0]
                
                # Check if data range is still too small compared to current plot range
                if data_range_with_padding < current_range * 0.3:  # Less than 30% of current range (increased from 15%)
                    center = (data_min + data_max) / 2
                    # Use a larger portion of the current range for next zoom
                    expanded_range = current_range * 0.5  # Use 50% of current range (increased from 30%)
                    y_min_dynamic = max(0, center - expanded_range / 2)
                    y_max_dynamic = min(current_ylim[1], center + expanded_range / 2)
                    ax.set_ylim(y_min_dynamic, y_max_dynamic)
                    iteration += 1
                else:
                    break  # Data is now reasonably visible
        
        # Add statistics text
        if len(all_values) > 0:
            mean_val = np.mean(all_values)
            std_val = np.std(all_values)
            ax.text(0.02, 0.98, f'Mean: {mean_val:.2f}\nStd: {std_val:.2f}', 
                    transform=ax.transAxes, verticalalignment='top',
                    bbox=dict(boxstyle='round', facecolor='white', alpha=0.8))
    
    # Hide unused subplots
    for i in range(len(metrics), len(axes)):
        axes[i].set_visible(False)
    
    plt.tight_layout()
    
    # Save plot
    save_path = os.path.join(output_dir, 'oracles_combined_metrics.pdf')
    plt.savefig(save_path, bbox_inches='tight', dpi=300)
    print(f"Saved combined oracles plot to {save_path}")
    #plt.show()

def plot_combined_tournament(df: pd.DataFrame, output_dir: str):
    """Create combined tournament plots showing metrics vs block numbers for each prover count"""
    if len(df) == 0 or 'num_provers' not in df.columns:
        print("No tournament data available")
        return
    
    # Get unique prover counts
    unique_provers = sorted(df['num_provers'].unique())
    
    # Define metrics to plot
    metrics = [
        ('sync_time_ms', 'Sync Time (s)', 0, 30),
        ('cpu_percent', 'CPU Usage (percent)', 0, 100),
        ('memory_bytes', 'Memory Usage (GB)', 0, 15),
        ('network_bytes_in', 'Network Bytes In (MB)', 0, 20),
        ('network_bytes_out', 'Network Bytes Out (MB)', 0, 3)
    ]
    
    # Create combined plots for each prover count
    for prover_count in unique_provers:
        display_provers = prover_count + 1
        subset = df[df['num_provers'] == prover_count]
        
        if len(subset) == 0:
            continue
            
        # Create figure with subplots
        fig, axes = plt.subplots(2, 3, figsize=(18, 12))
        fig.suptitle(f'Tournament Measurements - {display_provers} Provers', fontsize=16, fontweight='bold')
        
        # Flatten axes for easier indexing
        axes = axes.flatten()
        
        for i, (metric, title, y_min, y_max) in enumerate(metrics):
            if metric not in subset.columns:
                continue
                
            ax = axes[i]
            
            # Group by block number and calculate statistics
            block_stats = {}
            for block_num in sorted(subset['block_number'].unique()):
                block_data = subset[subset['block_number'] == block_num]
                values = block_data[metric].values
                
                # Convert units for better readability
                if metric == 'sync_time_ms':
                    values = values / 1000  # Convert to seconds
                elif metric == 'memory_bytes':
                    values = values / (1024 * 1024 * 1024)  # Convert to GB
                elif metric in ['network_bytes_in', 'network_bytes_out']:
                    values = values / (1024 * 1024)  # Convert to MB
                
                mean_val = np.mean(values)
                std_val = np.std(values)
                block_stats[block_num] = (mean_val, std_val)
            
            # Plot data
            block_numbers = sorted(block_stats.keys())
            means = [block_stats[b][0] for b in block_numbers]
            stds = [block_stats[b][1] for b in block_numbers]
            
            # Use blue for single prover plots
            color = '#1f77b4'
            
            # Plot with shaded standard deviation
            ax.fill_between(block_numbers, 
                          [m - s for m, s in zip(means, stds)],
                          [m + s for m, s in zip(means, stds)],
                          alpha=0.3, color=color)
            
            # Plot mean line
            ax.plot(block_numbers, means, 'o-', 
                   color=color, linewidth=2, markersize=6)
            
            ax.set_title(title, fontweight='bold')
            ax.set_xlabel('Block Number')
            ax.set_ylabel(title.split('(')[1].split(')')[0] if '(' in title else 'Value')
            ax.set_ylim(y_min, y_max)
            ax.grid(True, alpha=0.3)
            
            # Recursive dynamic Y-axis adjustment for low values
            if len(means) > 0:
                data_min = min(means)
                data_max = max(means)
                data_range = data_max - data_min
                
                # Include standard deviation in the range calculation
                if len(stds) > 0:
                    max_std = max(stds)
                    data_min_with_std = data_min - max_std
                    data_max_with_std = data_max + max_std
                    data_range_with_std = data_max_with_std - data_min_with_std
                else:
                    data_range_with_std = data_range
                
                # Keep zooming until we get a reasonable data-to-range ratio
                max_iterations = 3  # Reduce from 5 to 3
                iteration = 0
                current_y_min, current_y_max = y_min, y_max
                
                while iteration < max_iterations:
                    current_range = current_y_max - current_y_min
                    
                    # Check if data range is still too small compared to current plot range
                    if data_range_with_std < current_range * 0.3:  # Less than 30% of current range
                        center = (data_min + data_max) / 2
                        # Use a larger portion of the current range for next zoom
                        expanded_range = current_range * 0.5  # Use 50% of current range
                        y_min_dynamic = max(0, center - expanded_range / 2)
                        y_max_dynamic = min(y_max, center + expanded_range / 2)
                        current_y_min, current_y_max = y_min_dynamic, y_max_dynamic
                        iteration += 1
                    else:
                        break  # Data is now reasonably visible
                
                ax.set_ylim(current_y_min, current_y_max)
        
        # Hide unused subplots
        for i in range(len(metrics), len(axes)):
            axes[i].set_visible(False)
        
        plt.tight_layout()
        
        # Save plot
        save_path = os.path.join(output_dir, f'tournament_{display_provers}_provers_combined.pdf')
        plt.savefig(save_path, bbox_inches='tight', dpi=300)
        print(f"Saved {display_provers} provers combined plot to {save_path}")
        #plt.show()
    
    # Create final combined plots for all metrics
    final_metrics = [
        ('sync_time_ms', 'Tournament Sync Time vs Block Number - All Prover Configurations', 'Sync Time (s)', 0, 30),
        ('cpu_percent', 'Tournament CPU Usage vs Block Number - All Prover Configurations', 'CPU Usage (percent)', 0, 100),
        ('memory_bytes', 'Tournament Memory Usage vs Block Number - All Prover Configurations', 'Memory Usage (GB)', 0, 15),
        ('network_bytes_in', 'Tournament Network In vs Block Number - All Prover Configurations', 'Network Bytes In (MB)', 0, 20),
        ('network_bytes_out', 'Tournament Network Out vs Block Number - All Prover Configurations', 'Network Bytes Out (MB)', 0, 3)
    ]
    
    for metric, title, ylabel, y_min, y_max in final_metrics:
        if metric in df.columns:
            plot_final_metric_comparison(df, metric, title, ylabel, y_min, y_max, output_dir)

def plot_final_metric_comparison(df: pd.DataFrame, metric: str, title: str, ylabel: str, y_min: float, y_max: float, output_dir: str):
    """Create final plot showing a specific metric vs block numbers for all prover counts"""
    if len(df) == 0:
        return
    
    plt.figure(figsize=(12, 8))
    
    colors = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b']
    unique_provers = sorted(df['num_provers'].unique())
    
    all_means = []  # Collect all means to determine dynamic y-axis range
    
    for i, prover_count in enumerate(unique_provers):
        display_provers = prover_count + 1
        subset = df[df['num_provers'] == prover_count]
        
        if len(subset) == 0:
            continue
        
        # Group by block number and calculate statistics
        block_stats = {}
        for block_num in sorted(subset['block_number'].unique()):
            block_data = subset[subset['block_number'] == block_num]
            values = block_data[metric].values
            
            # Convert units for better readability
            if metric == 'sync_time_ms':
                values = values / 1000  # Convert to seconds
            elif metric == 'memory_bytes':
                values = values / (1024 * 1024 * 1024)  # Convert to GB
            elif metric in ['network_bytes_in', 'network_bytes_out']:
                values = values / (1024 * 1024)  # Convert to MB
            
            mean_val = np.mean(values)
            std_val = np.std(values)
            block_stats[block_num] = (mean_val, std_val)
        
        # Plot data
        block_numbers = sorted(block_stats.keys())
        means = [block_stats[b][0] for b in block_numbers]
        stds = [block_stats[b][1] for b in block_numbers]
        
        all_means.extend(means)  # Collect for y-axis calculation
        
        # Plot with shaded standard deviation
        plt.fill_between(block_numbers, 
                        [m - s for m, s in zip(means, stds)],
                        [m + s for m, s in zip(means, stds)],
                        alpha=0.2, color=colors[i % len(colors)])
        
        # Plot mean line
        plt.plot(block_numbers, means, 'o-', 
               color=colors[i % len(colors)], linewidth=3, markersize=8,
               label=f'{display_provers} provers')
    
    # Add reference line for sync time
    if metric == 'sync_time_ms':
        block_numbers = sorted(df['block_number'].unique())
        reference_values = [(10 * block_num) / 1000 for block_num in block_numbers]  # 10 ms per block.
        plt.plot(block_numbers, reference_values, '--', color='black', linewidth=2, 
               label='Reference: Full node', alpha=0.7)
    
    plt.xlabel('Block Number')
    plt.ylabel(ylabel)
    plt.title(title)
    
    # Recursive dynamic y-axis range based on actual data
    if all_means:
        data_min = min(all_means)
        data_max = max(all_means)
        data_range = data_max - data_min
        
        # Include standard deviation in the range calculation
        if len(stds) > 0:
            max_std = max(stds)
            data_min_with_std = data_min - max_std
            data_max_with_std = data_max + max_std
            data_range_with_std = data_max_with_std - data_min_with_std
        else:
            data_range_with_std = data_range
        
        # Keep zooming until we get a reasonable data-to-range ratio
        max_iterations = 3  # Reduce from 5 to 3
        iteration = 0
        current_y_min, current_y_max = y_min, y_max
        
        while iteration < max_iterations:
            current_range = current_y_max - current_y_min
            
            # Check if data range is still too small compared to current plot range
            if data_range_with_std < current_range * 0.3:  # Less than 30% of current range (increased from 15%)
                center = (data_min + data_max) / 2
                # Use a larger portion of the current range for next zoom
                expanded_range = current_range * 0.5  # Use 50% of current range (increased from 30%)
                y_min_dynamic = max(0, center - expanded_range / 2)
                y_max_dynamic = min(y_max, center + expanded_range / 2)
                current_y_min, current_y_max = y_min_dynamic, y_max_dynamic
                iteration += 1
            else:
                break  # Data is now reasonably visible
        
        plt.ylim(current_y_min, current_y_max)
    else:
        plt.ylim(y_min, y_max)
    
    plt.grid(True, alpha=0.3)
    plt.legend()
    
    # Create filename from metric name
    metric_name = metric.replace('_', '_')
    save_path = os.path.join(output_dir, f'tournament_final_{metric_name}_comparison.pdf')
    plt.savefig(save_path, bbox_inches='tight', dpi=300)
    print(f"Saved final {metric} comparison plot to {save_path}")
    #plt.show()

def plot_final_sync_time_comparison(df: pd.DataFrame, output_dir: str):
    """Create final plot showing sync time vs block numbers for all prover counts"""
    plot_final_metric_comparison(df, 'sync_time_ms', 'Tournament Sync Time vs Block Number - All Prover Configurations', 
                                'Sync Time (s)', 0, 30, output_dir)

def print_statistics(df: pd.DataFrame, data_type: str):
    """Print detailed statistics for the data"""
    if len(df) == 0:
        print(f"No data available for {data_type}")
        return
    
    print(f"\n=== {data_type.upper()} STATISTICS ===")
    
    if 'num_provers' in df.columns:
        unique_provers = df['num_provers'].unique()
        print(f"Number of different prover configurations: {len(unique_provers)}")
        for num_provers in sorted(unique_provers):
            subset = df[df['num_provers'] == num_provers]
            print(f"\n{num_provers} provers (n={len(subset)}):")
            for col in df.columns:
                if col in ['num_provers', 'block_number', 'iteration', 'timestamp']:
                    continue
                if col in subset.columns:
                    mean_val = subset[col].mean()
                    std_val = subset[col].std()
                    print(f"  {col}: {mean_val:.2f} ± {std_val:.2f}")
    else:
        print(f"Total measurements: {len(df)}")
        for col in df.columns:
            if col in ['block_number', 'iteration', 'timestamp']:
                continue
            if col in df.columns:
                mean_val = df[col].mean()
                std_val = df[col].std()
                print(f"  {col}: {mean_val:.2f} ± {std_val:.2f}")

def main():
    # Define data directories
    base_dir = Path(".")
    consensus_file = base_dir / "consensus" / "consensus_oracle_measurements.csv"
    execution_file = base_dir / "execution" / "execution_oracle_measurements.csv"
    tournament_file = base_dir / "tournament" / "tournament_measurements.csv"
    
    # Load data
    print("Loading measurement data...")
    consensus_df = load_csv_data(consensus_file)
    execution_df = load_csv_data(execution_file)
    tournament_df = load_csv_data(tournament_file)
    
    # Create output directories
    output_dirs = {
        'consensus': base_dir / "plots" / "consensus",
        'execution': base_dir / "plots" / "execution", 
        'tournament': base_dir / "plots" / "tournament",
        'oracles': base_dir / "plots" / "oracles"
    }
    
    for dir_path in output_dirs.values():
        dir_path.mkdir(parents=True, exist_ok=True)
    
    # Print statistics
    print_statistics(consensus_df, "consensus")
    print_statistics(execution_df, "execution")
    print_statistics(tournament_df, "tournament")
    
    # Generate plots
    print("\nGenerating plots...")
    
    # Combined oracle plots
    print("Creating combined oracle plots...")
    plot_combined_oracle_metrics(consensus_df, execution_df, output_dirs['oracles'])
    
    # Tournament plots (block-based only)
    print("Creating tournament plots...")
    plot_combined_tournament(tournament_df, output_dirs['tournament'])
    
    print("\nAll plots generated successfully!")
    print(f"Check the following directories for plots:")
    for data_type, dir_path in output_dirs.items():
        print(f"  {data_type}: {dir_path}")

if __name__ == "__main__":
    main()